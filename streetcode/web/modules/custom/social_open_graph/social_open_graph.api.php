<?php

/**
 * @file
 * Hooks provided by the Social Open Graph module.
 */

/**
 * @addtogroup hooks
 * @{
 */

/**
 * Provide a method to alter the array of text formats with convert URLs filter.
 *
 * @param array $formats
 *   List of text formats where a key is filter name and if a value is TRUE then
 *   the current format will use the filter for converting URLs.
 *
 * @ingroup social_open_graph_api
 *
 * @see \Drupal\social_open_graph\SocialOpenGraphConfigOverrideBase::loadOverrides()
 */
function hook_social_open_graph_formats_alter(array &$formats) {
  $formats['basic_html'] = FALSE;
}

/**
 * @} End of "addtogroup hooks".
 */
