FROM php:8.3-fpm

# Install system dependencies and PHP extensions required by Drupal
RUN apt-get update && apt-get install -y \
    git \
    curl \
    nginx \
    libpng-dev \
    libjpeg-dev \
    libwebp-dev \
    libonig-dev \
    libxml2-dev \
    libzip-dev \
    zip \
    unzip \
    libpq-dev \
    && docker-php-ext-configure gd --with-jpeg --with-webp \
    && docker-php-ext-install pdo pdo_pgsql mbstring exif pcntl bcmath gd zip opcache \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Composer
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

# Set working directory
WORKDIR /var/www/html

# Copy composer files
COPY composer.json composer.lock ./

# Install PHP dependencies
RUN composer install --no-dev --optimize-autoloader --no-interaction

# Copy PHP configuration
COPY php.ini /usr/local/etc/php/conf.d/drupal.ini

# Copy PHP-FPM pool configuration
COPY www.conf /usr/local/etc/php-fpm.d/www.conf

# Copy Nginx configuration
COPY nginx.conf /etc/nginx/sites-available/default
RUN rm -f /etc/nginx/sites-enabled/default \
    && ln -s /etc/nginx/sites-available/default /etc/nginx/sites-enabled/default

# Copy the rest of the application
COPY . .

# Set permissions for web directory
RUN chown -R www-data:www-data /var/www/html/web \
    && chmod -R 755 /var/www/html/web

# Expose port 80
EXPOSE 80

# Health check (curl is already installed above)
HEALTHCHECK --interval=30s --timeout=3s --start-period=40s --retries=3 \
    CMD curl -f http://localhost/ || exit 1

# Create startup script to run both PHP-FPM and Nginx
RUN echo '#!/bin/bash\n\
set -e\n\
\n\
# Start PHP-FPM in background\n\
php-fpm -D\n\
\n\
# Start Nginx in foreground\n\
exec nginx -g "daemon off;"' > /usr/local/bin/start.sh \
    && chmod +x /usr/local/bin/start.sh

# Start both PHP-FPM and Nginx
CMD ["/usr/local/bin/start.sh"]

