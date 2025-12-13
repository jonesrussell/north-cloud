<?php

namespace Drupal\social_open_graph\Service;

use Drupal\Core\Link;
use Drupal\Core\Url;
use Drupal\social_open_graph\UrlEmbed;

/**
 * Service for generating embed content.
 */
class EmbedGeneratorService {

  /**
   * The URL embed service.
   *
   * @var \Drupal\social_open_graph\UrlEmbed
   */
  protected UrlEmbed $urlEmbed;

  /**
   * Constructs an EmbedGeneratorService object.
   *
   * @param \Drupal\social_open_graph\UrlEmbed $url_embed
   *   The URL embed service.
   */
  public function __construct(UrlEmbed $url_embed) {
    $this->urlEmbed = $url_embed;
  }

  /**
   * Generates embed content for a URL.
   *
   * @param string $url
   *   The URL to generate embed content for.
   * @param string $uuid
   *   The unique identifier for the embed.
   *
   * @return array
   *   An array with 'type' (either 'iframe' or 'link') and 'content' keys.
   */
  public function generateEmbedContent(string $url, string $uuid): array {
    $info = $this->urlEmbed->getUrlInfo($url);

    if ($info && !empty($iframe = $info['code'])) {
      $provider = strtolower($info['providerName']);
      return [
        'type' => 'iframe',
        'content' => "<div id='social-open-graph-iframe-$uuid' class='social-open-graph-iframe-$provider'><p>$iframe</p></div>",
      ];
    }

    return [
      'type' => 'link',
      'content' => Link::fromTextAndUrl($url, Url::fromUri($url))->toString(),
    ];
  }

}

