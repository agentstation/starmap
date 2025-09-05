# Build stage - not needed since GoReleaser builds the binary
FROM alpine:3.20

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 starmap && \
    adduser -D -u 1000 -G starmap starmap

# Copy the pre-built binary from GoReleaser
COPY starmap /usr/local/bin/starmap

# Set ownership
RUN chown starmap:starmap /usr/local/bin/starmap

# Switch to non-root user
USER starmap

# Set working directory
WORKDIR /home/starmap

# Expose port if needed (not required for CLI tool)
# EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/starmap"]