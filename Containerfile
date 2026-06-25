#------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

# Update alpine.
RUN apk update && apk upgrade

# Install alpine dependencies.
RUN apk --no-cache --update add build-base bash

# Create and change to the 'service' directory.
WORKDIR /service

# Install project dependencies.
COPY go.mod go.sum ./
RUN go mod download

# Copy and build code.
COPY . .
RUN make build

#-------------------------------------------------------------------
FROM alpine:3

# Create and change to the the 'service' directory.
WORKDIR /service

# Copy the files to the production image from the builder stage.
COPY --from=builder /service/bin /service/
COPY --from=builder /service/db /service/db/

# Run the web service on container startup.
CMD ["/service/ingestor"]

#-------------------------------------------------------------------
