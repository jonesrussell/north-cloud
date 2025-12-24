FROM php:8.3-fpm

# Install system dependencies and PHP extensions required by Drupal
RUN apt-get update && apt-get install -y \
    git \
    curl \
    nginx \
    libpng-dev \
    libjpeg-dev \
    libwebp-dev \
    libavif-dev \
    libaom-dev \
    libdav1d-dev \
    libonig-dev \
    libxml2-dev \
    libzip-dev \
    zip \
    unzip \
    libpq-dev \
    postgresql-client \
    && docker-php-ext-configure gd --with-jpeg --with-webp --with-avif \
    && docker-php-ext-install pdo pdo_pgsql mbstring exif pcntl bcmath gd zip opcache \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install Node.js 20.x and npm
RUN curl -fsSL https://deb.nodesource.com/setup_25.x | bash - \
    && apt-get install -y nodejs \
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

# Build Tailwind CSS for the theme
RUN if [ -f "/var/www/html/web/themes/custom/streetcode_ui/package.json" ]; then \
    cd /var/www/html/web/themes/custom/streetcode_ui && \
    npm install && \
    npm run build && \
    rm -rf node_modules; \
    fi

# Set permissions for web directory
# Note: PHP-FPM and Nginx run as www-data (UID 33), which is standard
# This is acceptable as they're system users with limited privileges
RUN chown -R www-data:www-data /var/www/html/web \
    && chmod -R 755 /var/www/html/web \
    && chmod -R 775 /var/www/html/web/sites/default/files || true

# Expose port 80
EXPOSE 80

# Copy startup script
COPY start.sh /usr/local/bin/start.sh
RUN chmod +x /usr/local/bin/start.sh

# Health check (curl is already installed above)
HEALTHCHECK --interval=30s --timeout=3s --start-period=40s --retries=3 \
    CMD curl -f http://localhost/ || exit 1

# Start both PHP-FPM and Nginx
CMD ["/usr/local/bin/start.sh"]

