# Multi-stage build for LimeDB - Optimized for production
# Stage 1: Build the application using Amazon Corretto with build cache optimization
FROM amazoncorretto:21-alpine AS builder

# Metadata labels for better maintainability
LABEL maintainer="LimeDB Team" \
      description="LimeDB - Distributed Peer-to-Peer Database System" \
      version="0.0.1-SNAPSHOT" \
      java.version="21" \
      gradle.version="8.5"

# Build arguments for customization
ARG GRADLE_VERSION=8.5
ARG BUILD_PROFILE=default

WORKDIR /app

# Install build dependencies in a single layer
RUN apk add --no-cache curl unzip && \
    curl -L https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip -o gradle.zip && \
    unzip gradle.zip -d /opt && \
    mv /opt/gradle-${GRADLE_VERSION} /opt/gradle && \
    rm gradle.zip && \
    ln -s /opt/gradle/bin/gradle /usr/local/bin/gradle

# Set Gradle environment variables for optimization
ENV GRADLE_HOME=/opt/gradle
ENV PATH="/opt/gradle/bin:${PATH}"
ENV GRADLE_OPTS="-Dorg.gradle.daemon=false -Dorg.gradle.parallel=true -Dorg.gradle.configureondemand=true"

# Copy dependency files first for better Docker layer caching
COPY gradle/ gradle/
COPY gradlew gradlew.bat gradle.properties settings.gradle.kts ./
COPY app/build.gradle.kts app/

# Download dependencies in separate layer for better caching
RUN gradle :app:dependencies --no-daemon --quiet

# Copy source code
COPY app/src/ app/src/

# Build the application with optimizations
RUN gradle :app:bootJar --no-daemon --build-cache --parallel && \
    # Clean up Gradle cache to reduce layer size
    rm -rf ~/.gradle/caches/modules-*/modules-*/*.lock && \
    rm -rf ~/.gradle/caches/*/plugin-resolution/

# Stage 2: Optimized runtime image with JVM tuning
FROM amazoncorretto:21-alpine AS runtime

# Set JVM arguments as environment variables for flexibility
ENV JAVA_OPTS="-server \
    -XX:+UseG1GC \
    -XX:MaxGCPauseMillis=200 \
    -XX:+UseStringDeduplication \
    -XX:+OptimizeStringConcat \
    -XX:+UseCompressedOops \
    -XX:+UseCompressedClassPointers \
    -Djava.security.egd=file:/dev/./urandom \
    -Djava.awt.headless=true \
    -Dfile.encoding=UTF-8 \
    -Duser.timezone=UTC"

# Memory settings (can be overridden)
ENV JVM_MEMORY_OPTS="-Xms256m -Xmx512m -XX:MetaspaceSize=128m -XX:MaxMetaspaceSize=256m"

# Application-specific optimizations
ENV APP_OPTS="-Dspring.jmx.enabled=false \
    -Dspring.main.lazy-initialization=false \
    -Dspring.profiles.active=docker"

WORKDIR /app

# Install minimal runtime dependencies and create user in single layer
RUN apk add --no-cache curl dumb-init && \
    addgroup -S limedb && \
    adduser -S limedb -G limedb && \
    mkdir -p logs tmp && \
    chown -R limedb:limedb /app

# Copy the optimized jar from builder stage
COPY --from=builder --chown=limedb:limedb /app/app/build/libs/*.jar app.jar

# Use dumb-init for proper signal handling
ENTRYPOINT ["dumb-init", "--"]

# Switch to non-root user
USER limedb

# Expose the default port
EXPOSE 7001

# Enhanced health check using actuator endpoint
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -f http://localhost:7001/actuator/health || exit 1

# Optimized startup command with JVM tuning (using exec form for proper signal handling)
CMD ["sh", "-c", "exec java $JAVA_OPTS $JVM_MEMORY_OPTS $APP_OPTS -jar app.jar --server.port=7001 --node.id=${NODE_ID:-1} --spring.datasource.host=${DB_HOST:-localhost} --spring.datasource.port=${DB_PORT:-5432} --spring.datasource.username=${DB_USERNAME:-limedb} --spring.datasource.password=${DB_PASSWORD:-limedb}"]