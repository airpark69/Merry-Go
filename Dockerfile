# Base image
FROM golang:1.22.5

# Set the Current Working Directory inside the container
WORKDIR /app

# Install necessary build tools, Git, curl, and ffmpeg
RUN apt-get update && apt-get install -y \
    gcc g++ make git curl ffmpeg && \
    rm -rf /var/lib/apt/lists/*

RUN go install github.com/air-verse/air@latest

# Ensure Go bin directory is in PATH
ENV PATH="/go/bin:${PATH}"

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Expose port for the application
EXPOSE 18080

# Command to run Air for live reloading
CMD ["air", "-c", ".air.toml"]
