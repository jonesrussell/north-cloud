<?php

namespace Drupal\social_open_graph\Controller;

use Drupal\Component\Uuid\Uuid;
use Drupal\Core\Ajax\AjaxResponse;
use Drupal\Core\Ajax\ReplaceCommand;
use Drupal\Core\Controller\ControllerBase;
use Drupal\Core\Flood\FloodInterface;
use Drupal\Core\Site\Settings;
use Drupal\social_open_graph\Service\EmbedGeneratorService;
use Drupal\social_open_graph\SocialOpenGraphConstants;
use Symfony\Component\DependencyInjection\ContainerInterface;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpKernel\Exception\AccessDeniedHttpException;
use Symfony\Component\HttpKernel\Exception\BadRequestHttpException;
use Symfony\Component\HttpKernel\Exception\NotFoundHttpException;

/**
 * Returns responses for social_open_graph routes.
 */
class SocialOpenGraphController extends ControllerBase {

  /**
   * The embed generator service.
   *
   * @var \Drupal\social_open_graph\Service\EmbedGeneratorService
   */
  protected EmbedGeneratorService $embedGenerator;

  /**
   * The flood service.
   *
   * @var \Drupal\Core\Flood\FloodInterface
   */
  protected FloodInterface $flood;

  /**
   * The EmbedController constructor.
   *
   * @param \Drupal\social_open_graph\Service\EmbedGeneratorService $embed_generator
   *   The embed generator service.
   * @param \Drupal\Core\Flood\FloodInterface $flood
   *   The flood service.
   */
  public function __construct(EmbedGeneratorService $embed_generator, FloodInterface $flood) {
    $this->embedGenerator = $embed_generator;
    $this->flood = $flood;
  }

  /**
   * {@inheritdoc}
   *
   * @param \Symfony\Component\DependencyInjection\ContainerInterface $container
   *   The Drupal service container.
   *
   * @return static
   */
  public static function create(ContainerInterface $container) {
    return new static(
      $container->get('social_open_graph.embed_generator_service'),
      $container->get('flood')
    );
  }

  /**
   * Generates embed content of a give URL.
   *
   * When the site-wide setting for consent is enabled, the links in posts and
   * nodes will be replaced with placeholder divs and a show content button.
   *
   * Once user clicks the button, it will send request to this controller along
   * with url of the content to embed and an uuid which differentiates each
   * link.
   *
   * See:
   * 1. SocialOpenGraphConvertUrlToEmbedFilter::convertUrls
   * 2. SocialOpenGraphUrlEmbedFilter::process
   * 3. EmbedConsentForm::buildForm
   *
   * @param \Symfony\Component\HttpFoundation\Request $request
   *   The current request object.
   *
   * @return \Drupal\Core\Ajax\AjaxResponse
   *   The Ajax response.
   */
  public function generateEmbed(Request $request) {
    // Validate request parameters.
    $url = $this->validateAndSanitizeUrl($request->query->get('url'));
    $uuid = $this->validateUuid($request->query->get('uuid'));

    // Check flood protection.
    $this->checkFloodProtection();

    // Generate embed content.
    $embed = $this->embedGenerator->generateEmbedContent($url, $uuid);

    // Build and return AJAX response.
    return $this->buildAjaxResponse($uuid, $embed['content']);
  }

  /**
   * Validates and sanitizes a URL parameter.
   *
   * @param string|null $url
   *   The URL to validate.
   *
   * @return string
   *   The validated and sanitized URL.
   *
   * @throws \Symfony\Component\HttpKernel\Exception\BadRequestHttpException
   *   If the URL is invalid.
   */
  protected function validateAndSanitizeUrl(?string $url): string {
    if ($url === NULL || trim($url) === '') {
      throw new BadRequestHttpException('URL parameter is required.');
    }

    $url = trim($url);

    if (!filter_var($url, FILTER_VALIDATE_URL)) {
      throw new BadRequestHttpException('Invalid URL format.');
    }

    $parsed = parse_url($url);
    if (!isset($parsed['scheme']) || !in_array($parsed['scheme'], SocialOpenGraphConstants::ALLOWED_URL_SCHEMES)) {
      throw new BadRequestHttpException('Only HTTP/HTTPS URLs are allowed.');
    }

    if (strlen($url) > SocialOpenGraphConstants::URL_MAX_LENGTH) {
      throw new BadRequestHttpException('URL exceeds maximum length.');
    }

    return $url;
  }

  /**
   * Validates a UUID parameter.
   *
   * @param string|null $uuid
   *   The UUID to validate.
   *
   * @return string
   *   The validated UUID.
   *
   * @throws \Symfony\Component\HttpKernel\Exception\BadRequestHttpException
   *   If the UUID is invalid.
   */
  protected function validateUuid(?string $uuid): string {
    if ($uuid === NULL || !Uuid::isValid($uuid)) {
      throw new BadRequestHttpException('Invalid or missing UUID parameter.');
    }

    return $uuid;
  }

  /**
   * Checks flood protection and registers the event.
   *
   * @throws \Symfony\Component\HttpKernel\Exception\AccessDeniedHttpException
   *   If flood protection is triggered.
   */
  protected function checkFloodProtection(): void {
    $retries = Settings::get('social_open_graph_flood_retries', SocialOpenGraphConstants::FLOOD_RETRIES_DEFAULT);
    $timeWindow = Settings::get('social_open_graph_flood_time_window', SocialOpenGraphConstants::FLOOD_TIME_WINDOW_DEFAULT);

    if (!$this->flood->isAllowed('social_open_graph.generate_embed_flood_event', $retries, $timeWindow)) {
      throw new AccessDeniedHttpException('Too many requests. Please try again later.');
    }

    $this->flood->register('social_open_graph.generate_embed_flood_event', $timeWindow);
  }

  /**
   * Builds an AJAX response with the embed content.
   *
   * @param string $uuid
   *   The unique identifier for the embed.
   * @param string $content
   *   The embed content HTML.
   *
   * @return \Drupal\Core\Ajax\AjaxResponse
   *   The AJAX response.
   */
  protected function buildAjaxResponse(string $uuid, string $content): AjaxResponse {
    $selector = "#social-open-graph-iframe-$uuid";
    $response = new AjaxResponse();
    $response->addCommand(new ReplaceCommand($selector, $content));
    return $response;
  }

}
