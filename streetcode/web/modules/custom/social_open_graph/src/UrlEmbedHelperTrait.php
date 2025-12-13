<?php

namespace Drupal\social_open_graph;

use Drupal\Core\Extension\ModuleHandlerInterface;
use Drupal\url_embed\UrlEmbedInterface;

/**
 * Wrapper methods for URL embedding.
 *
 * This utility trait should only be used in application-level code, such as
 * classes that would implement ContainerInjectionInterface. Services registered
 * in the Container should not use this trait but inject the appropriate service
 * directly for easier testing.
 */
trait UrlEmbedHelperTrait {

  /**
   * The module handler service.
   *
   * @var \Drupal\Core\Extension\ModuleHandlerInterface
   */
  protected $moduleHandler;

  /**
   * The URL embed service.
   *
   * @var \Drupal\social_open_graph\UrlEmbed
   */
  protected $urlEmbed;

  /**
   * Returns the module handler.
   *
   * @return \Drupal\Core\Extension\ModuleHandlerInterface
   *   The module handler.
   */
  protected function moduleHandler() {
    if (!isset($this->moduleHandler)) {
      $this->moduleHandler = \Drupal::moduleHandler();
    }
    return $this->moduleHandler;
  }

  /**
   * Sets the module handler service.
   *
   * @param \Drupal\Core\Extension\ModuleHandlerInterface $module_handler
   *   The module handler service.
   *
   * @return \Drupal\Core\Extension\ModuleHandlerInterface
   *   The Module handler interface.
   */
  public function setModuleHandler(ModuleHandlerInterface $module_handler) {
    $this->moduleHandler = $module_handler;
    return $this;
  }

  /**
   * Returns the URL embed service.
   *
   * @return \Drupal\url_embed\UrlEmbedInterface
   *   The URL embed service..
   */
  protected function urlEmbed() {
    if (!isset($this->urlEmbed)) {
      $this->urlEmbed = \Drupal::service('social_open_graph');
    }
    return $this->urlEmbed;
  }

  /**
   * Sets the URL embed service.
   *
   * @param \Drupal\url_embed\UrlEmbedInterface $urlEmbed
   *   The URL embed service.
   *
   * @return \Drupal\url_embed\UrlEmbedInterface
   *   The URL embed service..
   */
  public function setUrlEmbed(UrlEmbedInterface $urlEmbed) {
    $this->urlEmbed = $urlEmbed;
    return $this;
  }

}
