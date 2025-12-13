<?php

namespace Drupal\social_open_graph;

use Drupal\Core\Config\ConfigFactoryInterface;
use Embed\Embed;
use Drupal\Core\Cache\Cache;
use Drupal\Core\Cache\CacheBackendInterface;
use Drupal\Component\Datetime\TimeInterface;
use Drupal\url_embed\UrlEmbedInterface;
use Embed\Extractor;


/**
 * A service class for handling URL embeds.
 */
class UrlEmbed implements UrlEmbedInterface {
  use UrlEmbedHelperTrait;

  /**
   * Drupal\Core\Cache\CacheBackendInterface definition.
   *
   * @var \Drupal\Core\Cache\CacheBackendInterface
   */
  protected $cacheBackend;

  /**
   * Drupal\Component\Datetime\TimeInterface.
   *
   * @var \Drupal\Component\Datetime\TimeInterface
   */
  protected $time;

  /**
   * Drupal\Core\Config\ConfigFactoryInterface definition.
   *
   * @var \Drupal\Core\Config\ConfigFactoryInterface
   */
  protected $configFactory;

  /**
   * The options passed to the adapter.
   *
   * @var array
   */
  public $config;

  /**
   * Constructs a UrlEmbed object.
   *
   * @param \Drupal\Core\Cache\CacheBackendInterface $cache_backend
   *   The cache backend factory.
   * @param \Drupal\Core\Datetime\TimeInterface $time
   *   The time factory.
   * @param \Drupal\Core\Config\ConfigFactoryInterface $config_factory
   *   (optional) The config factory.
   * @param array $config
   *   (optional) The options passed to the adapter.
   */
  public function __construct(CacheBackendInterface $cache_backend, TimeInterface $time, ConfigFactoryInterface $config_factory, array $config = []) {
    $this->cacheBackend = $cache_backend;
    $this->time = $time;
    $this->configFactory = $config_factory;
    $global_config = $config_factory->get('social_open_graph.settings');
    $defaults = [];

    if ($global_config->get('facebook_app_id') && $global_config->get('facebook_app_secret')) {
      $defaults['facebook']['key'] = $global_config->get('facebook_app_id') . '|' . $global_config->get('facebook_app_secret');
    }
    $this->config = array_replace_recursive($defaults, $config);
  }

  /**
   * Return the config factory.
   *
   * @{inheritdoc}
   */
  public function getConfig(): array {
    return $this->config;
  }

  /**
   * Set the config factory.
   *
   * @{inheritdoc}
   */
  public function setConfig(array $config): void {
    $this->config = $config;
  }

  /**
   * Return an Embed adapter with our Open Graph data.
   *
   * @{inheritdoc}
   */
  public function getEmbed(string $url, array $config = []): Extractor {
    $embed = new Embed();
    $embed->setSettings(array_replace_recursive($this->config, $config));
    return $embed->get($url);
  }

  /**
   * Return Open Graph data from requested URL.
   *
   * @{inheritdoc}
   */
  public function getUrlInfo(string $url): array {
    $data = [];
    $keys = [
      'aspectRatio',
      'code',
      'description',
      'height',
      'image',
      'providerName',
      'publishedTime',
      'title',
      'type',
      'width',
    ];
    $cid = 'social_open_graph:' . $url;
    $expire = $this->configFactory->get('social_open_graph.settings')->get('cache_expiration');
    if ($expire != 0 && $cache = $this->cacheBackend->get($cid)) {
      $data = $cache->data;
    }
    else {
      if ($info = $this->urlEmbed()->getEmbed($url)) {
        foreach ($keys as $key) {
          $data[$key] = $info->{$key};
        }
        if ($expire != 0) {
          $expiration = ($expire == Cache::PERMANENT) ? Cache::PERMANENT : $this->time->getRequestTime() + $expire;
          $this->cacheBackend->set($cid, $data, $expiration);
        }
      }
    }

    return $data;
  }

}
