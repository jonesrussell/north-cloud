<?php

namespace Drupal\social_open_graph\Service;

use Drupal\Core\Render\Renderer;
use Drupal\Core\Session\AccountProxyInterface;
use Drupal\filter\FilterProcessResult;
use Drupal\Component\Uuid\UuidInterface;

/**
 * Service class for Social Open Graph.
 */
class SocialOpenGraphHelper {

  /**
   * Uuid generator.
   *
   * @var \Drupal\Component\Uuid\UuidInterface
   */
  protected UuidInterface $uuidGenerator;

  /**
   * Current user object.
   *
   * @var \Drupal\Core\Session\AccountProxyInterface
   */
  protected AccountProxyInterface $currentUser;

  /**
   * Renderer services.
   *
   * @var \Drupal\Core\Render\Renderer
   */
  protected Renderer $renderer;

  /**
   * Constructor for SocialOpenGraphHelper.
   *
   * @param \Drupal\Component\Uuid\UuidInterface $uuid_generator
   *   The UUID generator.
   * @param \Drupal\Core\Session\AccountProxyInterface $current_user
   *   Current user object.
   * @param \Drupal\Core\Render\Renderer $renderer
   *   Renderer services.
   */
  public function __construct(UuidInterface $uuid_generator, AccountProxyInterface $current_user, Renderer $renderer) {
    $this->uuidGenerator = $uuid_generator;
    $this->currentUser = $current_user;
    $this->renderer = $renderer;
  }

  /**
   * Adds given cache tag and drupal ajax library.
   *
   * @param \Drupal\filter\FilterProcessResult $result
   *   FilterProcessResult object on which changes need to happen.
   * @param string $tag
   *   Tag to add.
   *
   * @return \Drupal\filter\FilterProcessResult
   *   The object itself.
   *
   * @see \Drupal\social_open_graph\Plugin\Filter\SocialEmbedConvertUrlToEmbedFilter
   * @see \Drupal\social_open_graph\Plugin\Filter\SocialEmbedUrlEmbedFilter
   */
  public function addDependencies(FilterProcessResult $result, string $tag): FilterProcessResult {
    // Add our custom tag so that we invalidate them when site manager
    // changes consent settings.
    // @see EmbedConsentForm
    $result->addCacheTags([$tag]);

    // Add user specific tag.
    $uid = $this->currentUser->id();
    $result->addCacheTags(["social_open_graph.filter.$uid"]);

    // We want to vary cache per user so the user settings can also be taken
    // into consent.
    $result->addCacheContexts(['user']);

    // We need this library to be attached as we are using 'use-ajax'
    // class in the show consent button markup.
    $result->addAttachments([
      'library' => [
        'core/drupal.ajax',
      ],
    ]);

    return $result;
  }

}
