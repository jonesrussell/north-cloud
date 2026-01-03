# Build stage
FROM node:25-alpine AS builder

WORKDIR /app

# Install dependencies using cached package files
COPY package*.json ./
RUN npm ci

# Copy source
COPY . .

# Build production assets
RUN npm run build

# Runtime stage
FROM nginx:alpine

# Remove default nginx config to avoid conflicts
RUN rm -rf /etc/nginx/conf.d/default.conf

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy SPA routing config
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
