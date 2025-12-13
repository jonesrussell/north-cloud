Social Open Graph
=================

Version: 1.0.0-alpha1

DESCRIPTION
-----------
Retrieve and store Open Graph data from websites. This module provides filter
plugins to embed URLs with Open Graph content in text formats.

REQUIREMENTS
------------
- Drupal 11
- embed module
- url_embed module
- twig_tweak module

INSTALLATION
------------
1. Place the module in your modules/custom directory
2. Enable the module via Drush: `drush en social_open_graph`
   Or via the admin UI: Extend > Install

FEATURES
--------
- Filter plugin to convert URLs to embeddable content
- Filter plugin to display Open Graph data
- Config override to automatically add filters to text formats
- REST API endpoint for Open Graph data
- Embed button for CKEditor

CONFIGURATION
-------------
The module automatically adds filter plugins to Basic HTML and Full HTML text
formats via config override. You can customize which formats use the filters
by implementing hook_social_open_graph_formats_alter().

UNINSTALLATION
--------------
Before uninstalling, you may need to temporarily disable the config override
service if you encounter "filter in use" errors. See the module's services.yml
file for details.

MAINTAINERS
-----------
Custom module for Streetcode project.
