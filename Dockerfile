# Multi-stage build for LimeDB
# Stage 1: Build the application
FROM gradle:8.5-jdk21 AS builder

WORKDIR /app

# Copy gradle files first for better caching
COPY gradle/ gradle/
COPY gradlew gradlew.bat gradle.properties settings.gradle.kts ./

# Copy source code
COPY app/ app/

# Build the application
RUN ./gradlew :app:bootJar --no-daemon

# Stage 2: Runtime image
FROM eclipse-temurin:21-jre

WORKDIR /app

# Install PostgreSQL client (optional, for debugging)
RUN apt-get update && apt-get install -y postgresql-client && rm -rf /var/lib/apt/lists/*

# Create a non-root user
RUN groupadd -r limedb && useradd -r -g limedb limedb

# Copy the built jar from builder stage
COPY --from=builder /app/app/build/libs/*.jar app.jar

# Create logs directory
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