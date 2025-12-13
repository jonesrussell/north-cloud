<?php

namespace Drupal\social_open_graph\Service;

use Drupal\Core\File\FileSystemInterface;
use Drupal\Core\Logger\LoggerChannelFactoryInterface;
use Drupal\social_open_graph\SocialOpenGraphConstants;
use GuzzleHttp\ClientInterface;
use GuzzleHttp\Exception\GuzzleException;
use Psr\Log\LoggerInterface;

/**
 * Service for downloading and validating images.
 */
class ImageDownloadService {

  /**
   * The HTTP client.
   *
   * @var \GuzzleHttp\ClientInterface
   */
  protected ClientInterface $httpClient;

  /**
   * The file system service.
   *
   * @var \Drupal\Core\File\FileSystemInterface
   */
  protected FileSystemInterface $fileSystem;

  /**
   * The logger service.
   *
   * @var \Psr\Log\LoggerInterface
   */
  protected LoggerInterface $logger;

  /**
   * Constructs an ImageDownloadService object.
   *
   * @param \GuzzleHttp\ClientInterface $http_client
   *   The HTTP client.
   * @param \Drupal\Core\Logger\LoggerChannelFactoryInterface $logger_factory
   *   The logger factory service.
   * @param \Drupal\Core\File\FileSystemInterface $file_system
   *   The file system service.
   */
  public function __construct(ClientInterface $http_client, LoggerChannelFactoryInterface $logger_factory, FileSystemInterface $file_system) {
    $this->httpClient = $http_client;
    $this->logger = $logger_factory->get('social_open_graph');
    $this->fileSystem = $file_system;
  }

  /**
   * Downloads and validates an image from a URL.
   *
   * @param string $image_url
   *   The URL of the image to download.
   *
   * @return string|null
   *   The image data as a string, or NULL if download/validation failed.
   */
  public function downloadAndValidateImage(string $image_url): ?string {
    // Validate URL scheme.
    $parsed = parse_url($image_url);
    if (!isset($parsed['scheme']) || !in_array($parsed['scheme'], SocialOpenGraphConstants::ALLOWED_URL_SCHEMES)) {
      $this->logger->warning('Invalid URL scheme for image: @url', ['@url' => $image_url]);
      return NULL;
    }

    // Use Guzzle with size limit.
    try {
      $response = $this->httpClient->get($image_url, [
        'timeout' => SocialOpenGraphConstants::IMAGE_DOWNLOAD_TIMEOUT,
        'max' => SocialOpenGraphConstants::IMAGE_MAX_SIZE,
      ]);

      $content_type = $response->getHeader('Content-Type');
      $content_type = !empty($content_type) ? $content_type[0] : '';

      // Extract MIME type (remove charset if present).
      $mime_type = strtok($content_type, ';');
      $mime_type = trim($mime_type);

      if (!in_array($mime_type, SocialOpenGraphConstants::ALLOWED_IMAGE_TYPES)) {
        $this->logger->warning('Invalid image type: @type for URL: @url', [
          '@type' => $mime_type,
          '@url' => $image_url,
        ]);
        return NULL;
      }

      return (string) $response->getBody();
    }
    catch (GuzzleException $e) {
      $this->logger->error('Failed to download image from @url: @message', [
        '@url' => $image_url,
        '@message' => $e->getMessage(),
      ]);
      return NULL;
    }
  }

}

