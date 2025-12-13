<?php

namespace Drupal\social_open_graph\Plugin\Filter;

use Drupal\file\Entity\File;
use Drupal\Component\Utility\Html;
use Drupal\Component\Uuid\UuidInterface;
use Drupal\Core\Config\ConfigFactoryInterface;
use Drupal\Core\Entity\EntityTypeManagerInterface;
use Drupal\Core\File\FileSystemInterface;
use Drupal\Core\Render\Renderer;
use Drupal\Core\Session\AccountProxyInterface;
use Drupal\filter\FilterProcessResult;
use Drupal\social_open_graph\Service\ImageDownloadService;
use Drupal\social_open_graph\Service\SocialOpenGraphHelper;
use Drupal\social_open_graph\SocialOpenGraphConstants;
use Drupal\url_embed\Plugin\Filter\UrlEmbedFilter;
use Drupal\url_embed\UrlEmbedInterface;
use Symfony\Component\DependencyInjection\ContainerInterface;

/**
 * Provides a filter to display embedded URLs based on data attributes.
 *
 * @Filter(
 *   id = "social_open_graph_url_embed",
 *   title = @Translation("Display embedded URLs Open Graph content"),
 *   description = @Translation("Embeds URLs using data attribute: data-embed-url."),
 *   type = Drupal\filter\Plugin\FilterInterface::TYPE_TRANSFORM_REVERSIBLE
 * )
 */
class SocialOpenGraphUrlEmbedFilter extends UrlEmbedFilter {

  /**
   * Uuid services.
   *
   * @var \Drupal\Component\Uuid\UuidInterface
   */
  protected UuidInterface $uuid;

  /**
   * The config factory services.
   *
   * @var \Drupal\Core\Config\ConfigFactoryInterface
   */
  protected ConfigFactoryInterface $configFactory;

  /**
   * The social embed helper services.
   *
   * @var \Drupal\social_open_graph\Service\SocialOpenGraphHelper
   */
  protected SocialOpenGraphHelper $embedHelper;

  /**
   * Current user object.
   *
   * @var \Drupal\Core\Session\AccountProxyInterface
   */
  protected AccountProxyInterface $currentUser;

  /**
   * The renderer service.
   *
   * @var \Drupal\Core\Render\Renderer
   */
  protected $renderer;

  /**
   * The entity type manager.
   *
   * @var \Drupal\Core\Entity\EntityTypeManagerInterface
   */
  protected EntityTypeManagerInterface $entityTypeManager;

  /**
   * The file system service.
   *
   * @var \Drupal\Core\File\FileSystemInterface
   */
  protected FileSystemInterface $fileSystem;

  /**
   * The URL embed service.
   *
   * @var \Drupal\url_embed\UrlEmbedInterface
   */
  protected $urlEmbed;

  /**
   * The image download service.
   *
   * @var \Drupal\social_open_graph\Service\ImageDownloadService
   */
  protected ImageDownloadService $imageDownloadService;

  /**
   * Constructs a SocialOpenGraphUrlEmbedFilter object.
   *
   * @param array $configuration
   *   A configuration array containing information about the plugin instance.
   * @param string $plugin_id
   *   The plugin ID for the plugin instance.
   * @param mixed $plugin_definition
   *   The plugin implementation definition.
   * @param \Drupal\url_embed\UrlEmbedInterface $url_embed
   *   The URL embed service.
   * @param \Drupal\Component\Uuid\UuidInterface $uuid
   *   The uuid services.
   * @param \Drupal\Core\Config\ConfigFactoryInterface $config_factory
   *   The config factory services.
   * @param \Drupal\Core\Render\Renderer $renderer
   *   The renderer services.
   * @param \Drupal\social_open_graph\Service\SocialOpenGraphHelper $embed_helper
   *   The social embed helper class object.
   * @param \Drupal\Core\Session\AccountProxyInterface $current_user
   *   Current user object.
   * @param \Drupal\Core\Entity\EntityTypeManagerInterface $entity_type_manager
   *   The entity type manager.
   * @param \Drupal\Core\File\FileSystemInterface $file_system
   *   The file system service.
   * @param \Drupal\social_open_graph\Service\ImageDownloadService $image_download_service
   *   The image download service.
   */
  public function __construct(
    array $configuration,
    $plugin_id,
    $plugin_definition,
    UrlEmbedInterface $url_embed,
    UuidInterface $uuid,
    ConfigFactoryInterface $config_factory,
    Renderer $renderer,
    SocialOpenGraphHelper $embed_helper,
    AccountProxyInterface $current_user,
    EntityTypeManagerInterface $entity_type_manager,
    FileSystemInterface $file_system,
    ImageDownloadService $image_download_service
  ) {
    parent::__construct($configuration, $plugin_id, $plugin_definition, $url_embed, $renderer);
    $this->uuid = $uuid;
    $this->configFactory = $config_factory;
    $this->renderer = $renderer;
    $this->embedHelper = $embed_helper;
    $this->currentUser = $current_user;
    $this->entityTypeManager = $entity_type_manager;
    $this->fileSystem = $file_system;
    $this->urlEmbed = $url_embed;
    $this->imageDownloadService = $image_download_service;
  }

  /**
   * {@inheritdoc}
   */
  public static function create(ContainerInterface $container, array $configuration, $plugin_id, $plugin_definition) {
    return new static(
      $configuration,
      $plugin_id,
      $plugin_definition,
      $container->get('social_open_graph'),
      $container->get('uuid'),
      $container->get('config.factory'),
      $container->get('renderer'),
      $container->get('social_open_graph.helper_service'),
      $container->get('current_user'),
      $container->get('entity_type.manager'),
      $container->get('file_system'),
      $container->get('social_open_graph.image_download_service')
    );
  }

  /**
   * {@inheritdoc}
   */
  public function process($text, $langcode) {
    $result = new FilterProcessResult($text);
    if (strpos($text, SocialOpenGraphConstants::EMBED_ATTRIBUTE) !== FALSE) {
      $dom = Html::load($text);
      /** @var \DOMXPath $xpath */
      $xpath = new \DOMXPath($dom);
      /** @var \DOMNode[] $matching_nodes */
      $matching_nodes = $xpath->query('//' . SocialOpenGraphConstants::EMBED_TAG . '[@' . SocialOpenGraphConstants::EMBED_ATTRIBUTE . ']');

      if ($matching_nodes->length === 0) {
        return $result;
      }

      // Collect all URLs first to optimize database queries.
      $urls_to_process = [];
      foreach ($matching_nodes as $node) {
        /** @var \DOMElement $node */
        $url = $node->getAttribute('data-embed-url');
        if ($url) {
          $urls_to_process[] = $url;
        }
      }

      // Load all existing entities at once to avoid N+1 query problem.
      $storage = $this->entityTypeManager->getStorage('social_open_graph_url');
      $existing_entities = $storage->loadByProperties(['url' => $urls_to_process]);

      // Create lookup array for quick access and collect file IDs.
      $entity_lookup = [];
      $file_ids = [];
      foreach ($existing_entities as $entity) {
        $entity_lookup[$entity->get('url')->value] = $entity;
        $info = $entity->toArray();
        if (!empty($info['image'][0]['target_id'])) {
          $file_ids[] = $info['image'][0]['target_id'];
        }
      }

      // Batch load all files at once to avoid N+1 query problem.
      $files = !empty($file_ids) ? File::loadMultiple($file_ids) : [];

      // Process each node.
      foreach ($matching_nodes as $node) {
        /** @var \DOMElement $node */
        $url = $node->getAttribute(SocialOpenGraphConstants::EMBED_ATTRIBUTE);
        if (!$url) {
          continue;
        }

        $providerName = NULL;
        $title = NULL;
        $description = NULL;
        $file = NULL;

        // Check if entity already exists.
        if (isset($entity_lookup[$url])) {
          $entity = $entity_lookup[$url];
          $info = $entity->toArray();

          $providerName = $info['provider_name'][0]['value'] ?? NULL;
          $title = $info['title'][0]['value'] ?? NULL;
          $description = $info['description'][0]['value'] ?? NULL;
          if (!empty($info['image'][0]['target_id'])) {
            $file = $files[$info['image'][0]['target_id']] ?? NULL;
          }
        }
        else {
          // Create new Open Graph entity.
          try {
            $info = $this->urlEmbed->getUrlInfo($url);
            if (!$info) {
              \Drupal::logger('social_open_graph')->warning('Failed to get URL info for @url', ['@url' => $url]);
              continue;
            }

            // Validate image URL exists.
            if (empty($info['image'])) {
              \Drupal::logger('social_open_graph')->warning('No image URL found for @url', ['@url' => $url]);
              continue;
            }

            // Download and validate image.
            $image_data = $this->imageDownloadService->downloadAndValidateImage($info['image']);
            if ($image_data === NULL) {
              \Drupal::logger('social_open_graph')->warning('Failed to download or validate image from @image_url', ['@image_url' => $info['image']]);
              continue;
            }

            // Sanitize filename properly.
            $parsed_url = parse_url($info['image']);
            $original_filename = basename($parsed_url['path'] ?? 'image.jpg');
            $safe_filename = preg_replace('/[^a-zA-Z0-9._-]/', '_', $original_filename);
            $destination = 'public://social_open_graph_' . uniqid() . '_' . $safe_filename;

            // Save file using file_save_data.
            $file = file_save_data($image_data, $destination, FileSystemInterface::EXISTS_RENAME);

            if (!$file) {
              \Drupal::logger('social_open_graph')->error('Failed to save image file for @url', ['@url' => $url]);
              continue;
            }

            $title = $info['title'] ?? NULL;
            $providerName = $info['providerName'] ?? NULL;
            $description = $info['description'] ?? NULL;

            $urlOpenGraph = $storage->create([
              'url' => $url,
              'title' => $title,
              'image' => [
                'target_id' => $file->id(),
                'alt' => 'Article image',
              ],
              'provider_name' => $providerName,
              'description' => $description,
            ]);

            $urlOpenGraph->save();
          }
          catch (\Exception $e) {
            \Drupal::logger('social_open_graph')->error('Error processing image: @message', ['@message' => $e->getMessage()]);
            continue;
          }
        }

        // Skip if we don't have a file to render.
        if (!$file) {
          continue;
        }

        // Render Open Graph entity.
        $uri = $file->getFileUri();
        $renderable = [
          '#theme' => 'open_graph',
          '#url' => $url,
          '#image' => $uri,
          '#providerName' => $providerName,
          '#title' => $title,
          '#description' => $description,
        ];
        $url_output = $this->renderer->renderPlain($renderable);
        $this->replaceNodeContent($node, $url_output);
      }

      $result->setProcessedText(Html::serialize($dom));
    }
    // Add the required dependencies and cache tags.
    return $this->addFilterDependencies($result, 'social_open_graph:filter.url_embed');
  }

  /**
   * Adds filter dependencies to the result.
   *
   * @param \Drupal\filter\FilterProcessResult $result
   *   The filter process result.
   * @param string $tag
   *   The cache tag to add.
   *
   * @return \Drupal\filter\FilterProcessResult
   *   The filter process result with dependencies added.
   */
  protected function addFilterDependencies(FilterProcessResult $result, string $tag): FilterProcessResult {
    return $this->embedHelper->addDependencies($result, $tag);
  }

}
