# Multi-stage build for LimeDB
# Stage 1: Build the application using Amazon Corretto
FROM amazoncorretto:21 AS builder

WORKDIR /app

# Install curl and unzip for gradle
RUN yum update -y && yum install -y curl unzip && yum clean all

# Install Gradle manually
RUN curl -L https://services.gradle.org/distributions/gradle-8.5-bin.zip -o gradle.zip && \
    unzip gradle.zip && \
    mv gradle-8.5 /opt/gradle && \
    rm gradle.zip
ENV PATH="/opt/gradle/bin:${PATH}"

# Copy gradle files first for better caching
COPY gradle/ gradle/
COPY gradlew gradlew.bat gradle.properties settings.gradle.kts ./

# Copy source code
COPY app/ app/

# Build the application
RUN gradle :app:bootJar --no-daemon

# Stage 2: Runtime image
FROM ubuntu:22.04

WORKDIR /app

# Install OpenJDK 21 and curl for health checks
RUN apt-get update && \
    apt-get install -y openjdk-21-jre curl && \
    rm -rf /var/lib/apt/lists/*

# Create a non-root user
RUN groupadd -r limedb && useradd -r -g limedb limedb

# Copy the built jar from builder stage
COPY --from=builder /app/app/build/libs/*.jar app.jar

# Create logs directory and set permissions
RUN mkdir -p logs && chown -R limedb:limedb /app

# Switch to non-root user
USER limedb

# Expose the default port
EXPOSE 7001

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:7001/cluster/state || exit 1

# Default command
ENTRYPOINT ["java", "-jar", "app.jar"]
CMD ["--server.port=7001", "--node.id=1"]