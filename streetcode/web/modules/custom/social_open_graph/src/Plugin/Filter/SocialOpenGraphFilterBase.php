<?php

namespace Drupal\social_open_graph\Plugin\Filter;

use Drupal\Core\Plugin\ContainerFactoryPluginInterface;
use Drupal\filter\FilterBase;
use Drupal\filter\FilterProcessResult;
use Drupal\social_open_graph\Service\SocialOpenGraphHelper;
use Symfony\Component\DependencyInjection\ContainerInterface;

/**
 * Base class for Social Open Graph filter plugins.
 */
abstract class SocialOpenGraphFilterBase extends FilterBase implements ContainerFactoryPluginInterface {

  /**
   * The social embed helper services.
   *
   * @var \Drupal\social_open_graph\Service\SocialOpenGraphHelper
   */
  protected SocialOpenGraphHelper $embedHelper;

  /**
   * Constructs a SocialOpenGraphFilterBase object.
   *
   * @param array $configuration
   *   A configuration array containing information about the plugin instance.
   * @param string $plugin_id
   *   The plugin ID for the plugin instance.
   * @param mixed $plugin_definition
   *   The plugin implementation definition.
   * @param \Drupal\social_open_graph\Service\SocialOpenGraphHelper $embed_helper
   *   The social embed helper class object.
   */
  public function __construct(array $configuration, $plugin_id, $plugin_definition, SocialOpenGraphHelper $embed_helper) {
    parent::__construct($configuration, $plugin_id, $plugin_definition);
    $this->embedHelper = $embed_helper;
  }

  /**
   * {@inheritdoc}
   */
  public static function create(ContainerInterface $container, array $configuration, $plugin_id, $plugin_definition) {
    return new static(
      $configuration,
      $plugin_id,
      $plugin_definition,
      $container->get('social_open_graph.helper_service')
    );
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

