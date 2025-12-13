<?php

namespace Drupal\social_open_graph\Plugin\Filter;

use Drupal\Component\Utility\Html;
use Drupal\Core\Form\FormStateInterface;
use Drupal\Core\Plugin\ContainerFactoryPluginInterface;
use Drupal\filter\FilterProcessResult;
use Drupal\social_open_graph\Service\SocialOpenGraphHelper;
use Drupal\social_open_graph\SocialOpenGraphConstants;
use Drupal\url_embed\Plugin\Filter\ConvertUrlToEmbedFilter;
use Drupal\url_embed\UrlEmbedInterface;
use Symfony\Component\DependencyInjection\ContainerInterface;

/**
 * Provides a filter to display embedded entities based on data attributes.
 *
 * @Filter(
 *   id = "social_open_graph_convert_url",
 *   title = @Translation("Convert SUPPORTED URLs to URL embeds"),
 *   description = @Translation("Convert only URLs that are supported to URL embeds."),
 *   type = Drupal\filter\Plugin\FilterInterface::TYPE_TRANSFORM_REVERSIBLE,
 *   settings = {
 *     "url_prefix" = "",
 *   },
 * )
 */
class SocialOpenGraphConvertUrlToEmbedFilter extends ConvertUrlToEmbedFilter implements ContainerFactoryPluginInterface {

  /**
   * The URL embed service.
   *
   * @var \Drupal\url_embed\UrlEmbedInterface|null
   */
  protected ?UrlEmbedInterface $urlEmbed = NULL;

  /**
   * The social embed helper services.
   *
   * @var \Drupal\social_open_graph\Service\SocialOpenGraphHelper|null
   */
  protected ?SocialOpenGraphHelper $embedHelper = NULL;

  /**
   * Constructs a SocialOpenGraphConvertUrlToEmbedFilter object.
   *
   * @param array $configuration
   *   A configuration array containing information about the plugin instance.
   * @param string $plugin_id
   *   The plugin ID for the plugin instance.
   * @param mixed $plugin_definition
   *   The plugin implementation definition.
   * @param \Drupal\url_embed\UrlEmbedInterface|null $url_embed
   *   The URL embed service (optional, will be lazy-loaded if not provided).
   * @param \Drupal\social_open_graph\Service\SocialOpenGraphHelper|null $embed_helper
   *   The social embed helper class object (optional, will be lazy-loaded if not provided).
   */
  public function __construct(array $configuration, $plugin_id, $plugin_definition, ?UrlEmbedInterface $url_embed = NULL, ?SocialOpenGraphHelper $embed_helper = NULL) {
    parent::__construct($configuration, $plugin_id, $plugin_definition);
    $this->urlEmbed = $url_embed;
    $this->embedHelper = $embed_helper;
  }

  /**
   * Gets the URL embed service, lazy-loading if necessary.
   *
   * @return \Drupal\url_embed\UrlEmbedInterface
   *   The URL embed service.
   */
  protected function getUrlEmbed(): UrlEmbedInterface {
    if ($this->urlEmbed === NULL) {
      $this->urlEmbed = \Drupal::service('social_open_graph');
    }
    return $this->urlEmbed;
  }

  /**
   * Gets the embed helper service, lazy-loading if necessary.
   *
   * @return \Drupal\social_open_graph\Service\SocialOpenGraphHelper
   *   The embed helper service.
   */
  protected function getEmbedHelper(): SocialOpenGraphHelper {
    if ($this->embedHelper === NULL) {
      $this->embedHelper = \Drupal::service('social_open_graph.helper_service');
    }
    return $this->embedHelper;
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
      $container->get('social_open_graph.helper_service')
    );
  }

  /**
   * {@inheritdoc}
   */
  public function settingsForm(array $form, FormStateInterface $form_state) {
    return [];
  }

  /**
   * {@inheritdoc}
   */
  public function process($text, $langcode) {
    // Check if consent is enabled - if not, use parent's convertUrls.
    $config = \Drupal::config('social_open_graph.settings');
    if (!$config->get('settings')) {
      $result = new FilterProcessResult(parent::convertUrls($text, $this->settings['url_prefix']));
    }
    else {
      $result = new FilterProcessResult($this->convertUrlsWithService($text, $this->settings['url_prefix']));
    }
    // Add the required dependencies and cache tags.
    return $this->addFilterDependencies($result, 'social_open_graph:filter.convert_url');
  }

  /**
   * Replaces appearances of supported URLs with placeholder embed elements.
   *
   * Logic of this function is copied from _filter_url() and slightly adopted
   * for our use case. _filter_url() is unfortunately not general enough to
   * re-use it.
   *
   * @param string $text
   *   Text to be processed.
   * @param string $url_prefix
   *   (Optional) Prefix that should be used to manually choose which URLs
   *   should be converted.
   *
   * @return mixed
   *   Processed text.
   */
  protected function convertUrlsWithService($text, $url_prefix = '') {
    // Tags to skip and not recurse into.
    $ignore_tags = 'a|script|style|code|pre';

    // Prepare protocols pattern for absolute URLs.
    $protocols = \Drupal::getContainer()->getParameter('filter_protocols');
    $protocols = implode(':(?://)?|', $protocols) . ':(?://)?';

    $valid_url_path_characters = "[\p{L}\p{M}\p{N}!\*\';:=\+,\.\$\/%#\[\]\-_~@&]";

    // Allow URL paths to contain balanced parens.
    $valid_url_balanced_parens = '\(' . $valid_url_path_characters . '+\)';

    // Valid end-of-path characters.
    $valid_url_ending_characters = '[\p{L}\p{M}\p{N}:_+~#=/]|(?:' . $valid_url_balanced_parens . ')';

    $valid_url_query_chars = '[a-zA-Z0-9!?\*\'@\(\);:&=\+\$\/%#\[\]\-_\.,~|]';
    $valid_url_query_ending_chars = '[a-zA-Z0-9_&=#\/]';

    // Full path.
    $valid_url_path = '(?:(?:' . $valid_url_path_characters . '*(?:' . $valid_url_balanced_parens . $valid_url_path_characters . '*)*' . $valid_url_ending_characters . ')|(?:@' . $valid_url_path_characters . '+\/))';

    // Prepare domain name pattern.
    $domain = '(?:[\p{L}\p{M}\p{N}._+-]+\.)?[\p{L}\p{M}]{2,64}\b';
    $ip = '(?:[0-9]{1,3}\.){3}[0-9]{1,3}';
    $auth = '[\p{L}\p{M}\p{N}:%_+*~#?&=.,/;-]+@';
    $trail = '(' . $valid_url_path . '*)?(\\?' . $valid_url_query_chars . '*' . $valid_url_query_ending_chars . ')?';

    // Match absolute URLs.
    $url_pattern = "(?:$auth)?(?:$domain|$ip)/?(?:$trail)?";
    $pattern = "`$url_prefix((?:$protocols)(?:$url_pattern))`u";

    // HTML comments need to be handled separately.
    _filter_url_escape_comments([], TRUE);
    $text = preg_replace_callback('`<!--(.*?)-->`s', '_filter_url_escape_comments', $text) ?? $text;

    // Split at all tags.
    $chunks = preg_split('/(<.+?>)/is', $text, -1, PREG_SPLIT_DELIM_CAPTURE);
    $chunk_type = 'text';
    $open_tag = '';

    // Capture $this for use in closure.
    $url_embed = $this->getUrlEmbed();

    for ($i = 0; $i < count($chunks); $i++) {
      if ($chunk_type == 'text') {
        // Only process this text if there are no unclosed $ignore_tags.
        if ($open_tag == '') {
          $chunks[$i] = preg_replace_callback(
            $pattern,
            function ($match) use ($url_embed) {
              try {
                $info = $url_embed->getUrlInfo(Html::decodeEntities($match[1]));
                if ($info) {
                  return '<' . SocialOpenGraphConstants::EMBED_TAG . ' ' . SocialOpenGraphConstants::EMBED_ATTRIBUTE . '="' . $match[1] . '"></' . SocialOpenGraphConstants::EMBED_TAG . '>';
                }
                else {
                  return $match[1];
                }
              }
              catch (\Exception $e) {
                // If anything goes wrong, log and leave URL as is.
                \Drupal::logger('social_open_graph')->error('Error converting URL @url: @message', [
                  '@url' => $match[1],
                  '@message' => $e->getMessage(),
                ]);
                return $match[1];
              }
            },
            $chunks[$i]
          );
        }
        $chunk_type = 'tag';
      }
      else {
        // Only process this tag if there are no unclosed $ignore_tags.
        if ($open_tag == '') {
          if (preg_match("`<($ignore_tags)(?:\s|>)`i", $chunks[$i], $matches)) {
            $open_tag = $matches[1];
          }
        }
        else {
          if (preg_match("`<\/$open_tag>`i", $chunks[$i], $matches)) {
            $open_tag = '';
          }
        }
        $chunk_type = 'text';
      }
    }

    $text = implode($chunks);
    // Revert to the original comment contents.
    _filter_url_escape_comments([], FALSE);
    return preg_replace_callback('`<!--(.*?)-->`', '_filter_url_escape_comments', $text);
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
    return $this->getEmbedHelper()->addDependencies($result, $tag);
  }

}
