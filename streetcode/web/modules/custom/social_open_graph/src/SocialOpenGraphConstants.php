<?php

namespace Drupal\social_open_graph;

/**
 * Constants for the Social Open Graph module.
 */
class SocialOpenGraphConstants {

  /**
   * Default number of flood retries.
   */
  public const FLOOD_RETRIES_DEFAULT = 50;

  /**
   * Default time window for flood control (in seconds).
   */
  public const FLOOD_TIME_WINDOW_DEFAULT = 300;

  /**
   * Maximum URL length.
   */
  public const URL_MAX_LENGTH = 2083;

  /**
   * Maximum title length.
   */
  public const TITLE_MAX_LENGTH = 255;

  /**
   * Embed attribute name.
   */
  public const EMBED_ATTRIBUTE = 'data-embed-url';

  /**
   * Embed tag name.
   */
  public const EMBED_TAG = 'drupal-url';

  /**
   * Maximum image file size in bytes (5MB).
   */
  public const IMAGE_MAX_SIZE = 5 * 1024 * 1024;

  /**
   * Image download timeout in seconds.
   */
  public const IMAGE_DOWNLOAD_TIMEOUT = 10;

  /**
   * Allowed image MIME types.
   */
  public const ALLOWED_IMAGE_TYPES = [
    'image/jpeg',
    'image/png',
    'image/gif',
    'image/webp',
  ];

  /**
   * Allowed URL schemes.
   */
  public const ALLOWED_URL_SCHEMES = ['http', 'https'];

}

