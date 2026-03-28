# northcloud-search FTS5 Migration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the ES-proxy search frontend with a self-contained Waaseyaa app that ingests content from North Cloud's Redis pub/sub and indexes it into SQLite FTS5.

**Architecture:** A Redis subscriber command ingests all pipeline content (articles, recipes, jobs, RFPs) into FTS5 via Waaseyaa's search package. Controllers query FTS5 directly instead of proxying to north-cloud's search service. Two containers: web (PHP built-in server) + subscriber (long-running console command) sharing a SQLite volume.

**Tech Stack:** PHP 8.4, Waaseyaa framework (search, foundation, cli packages), predis/predis for Redis, SQLite FTS5, Twig templates

**Repo:** `/home/jones/dev/northcloud-search` (GitHub: `waaseyaa/northcloud-search`)

**Reference files:**
- Waaseyaa search package: `/home/jones/dev/waaseyaa/packages/search/src/`
- Redis message format: `/home/jones/dev/north-cloud/publisher/docs/REDIS_MESSAGE_FORMAT.md`
- Design spec: `/home/jones/dev/north-cloud/docs/superpowers/specs/2026-03-21-northcloud-search-waaseyaa-design.md`

---

### Task 1: Add predis dependency and ContentDocument class

**Files:**
- Modify: `composer.json`
- Create: `src/Document/ContentDocument.php`
- Create: `tests/Document/ContentDocumentTest.php`

- [ ] **Step 1: Add predis/predis to composer.json**

Add the Redis client dependency:

```bash
cd /home/jones/dev/northcloud-search
composer require predis/predis
```

- [ ] **Step 2: Write the failing test for ContentDocument**

Create `tests/Document/ContentDocumentTest.php`:

```php
<?php

declare(strict_types=1);

namespace App\Tests\Document;

use App\Document\ContentDocument;
use PHPUnit\Framework\TestCase;

final class ContentDocumentTest extends TestCase
{
    public function testFromRedisMessageMapsArticle(): void
    {
        $message = [
            'id' => 'es-doc-123',
            'title' => 'Police Investigate Break-In',
            'body' => 'Full article text here.',
            'canonical_url' => 'https://example.com/article',
            'published_date' => '2026-03-28T10:00:00Z',
            'quality_score' => 85,
            'topics' => ['crime', 'local_news'],
            'content_type' => 'article',
            'og_image' => 'https://example.com/image.jpg',
            'source' => 'https://example.com/original',
        ];

        $doc = ContentDocument::fromRedisMessage($message);

        $this->assertSame('es-doc-123', $doc->getSearchDocumentId());

        $searchDoc = $doc->toSearchDocument();
        $this->assertSame('Police Investigate Break-In', $searchDoc['title']);
        $this->assertSame('Full article text here.', $searchDoc['body']);

        $meta = $doc->toSearchMetadata();
        $this->assertSame('content', $meta['entity_type']);
        $this->assertSame('article', $meta['content_type']);
        $this->assertSame('example.com', $meta['source_name']);
        $this->assertSame(85, $meta['quality_score']);
        $this->assertSame(['crime', 'local_news'], $meta['topics']);
        $this->assertSame('https://example.com/article', $meta['url']);
        $this->assertSame('https://example.com/image.jpg', $meta['og_image']);
    }

    public function testFromRedisMessageExtractsSourceDomain(): void
    {
        $doc = ContentDocument::fromRedisMessage([
            'id' => 'doc-1',
            'title' => 'Test',
            'body' => 'Body',
            'canonical_url' => 'https://www.cbc.ca/news/article',
            'source' => 'https://www.cbc.ca/feed',
            'published_date' => '2026-03-28T10:00:00Z',
            'quality_score' => 50,
            'topics' => [],
            'content_type' => 'article',
        ]);

        $this->assertSame('cbc.ca', $doc->toSearchMetadata()['source_name']);
    }

    public function testFromRedisMessageHandlesMissingFields(): void
    {
        $doc = ContentDocument::fromRedisMessage([
            'id' => 'doc-2',
            'title' => 'Minimal',
            'canonical_url' => 'https://example.com',
        ]);

        $searchDoc = $doc->toSearchDocument();
        $this->assertSame('Minimal', $searchDoc['title']);
        $this->assertSame('', $searchDoc['body']);

        $meta = $doc->toSearchMetadata();
        $this->assertSame('', $meta['content_type']);
        $this->assertSame(0, $meta['quality_score']);
        $this->assertSame([], $meta['topics']);
    }

    public function testFromRedisMessageUsesRawTextFallback(): void
    {
        $doc = ContentDocument::fromRedisMessage([
            'id' => 'doc-3',
            'title' => 'Test',
            'raw_text' => 'Text from raw_text field',
            'canonical_url' => 'https://example.com',
        ]);

        $this->assertSame('Text from raw_text field', $doc->toSearchDocument()['body']);
    }

    public function testFromRedisMessageStoresMetadataJson(): void
    {
        $message = [
            'id' => 'doc-4',
            'title' => 'Mining News',
            'canonical_url' => 'https://example.com',
            'content_type' => 'article',
            'mining' => ['relevance' => 'core_mining', 'commodities' => ['gold']],
            'crime_relevance' => 'not_crime',
        ];

        $doc = ContentDocument::fromRedisMessage($message);
        $meta = $doc->toSearchMetadata();

        $this->assertIsString($meta['metadata_json']);
        $decoded = json_decode($meta['metadata_json'], true);
        $this->assertSame('core_mining', $decoded['mining']['relevance']);
    }
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /home/jones/dev/northcloud-search
vendor/bin/phpunit tests/Document/ContentDocumentTest.php
```

Expected: FAIL — class `App\Document\ContentDocument` not found.

- [ ] **Step 4: Implement ContentDocument**

Create `src/Document/ContentDocument.php`:

```php
<?php

declare(strict_types=1);

namespace App\Document;

use Waaseyaa\Search\SearchIndexableInterface;

final readonly class ContentDocument implements SearchIndexableInterface
{
    /**
     * @param string[] $topics
     */
    private function __construct(
        private string $id,
        private string $title,
        private string $body,
        private string $url,
        private string $contentType,
        private string $sourceName,
        private int $qualityScore,
        private array $topics,
        private string $ogImage,
        private string $publishedAt,
        private string $metadataJson,
    ) {}

    /**
     * @param array<string, mixed> $data Redis pub/sub message
     */
    public static function fromRedisMessage(array $data): self
    {
        $sourceUrl = (string) ($data['source'] ?? $data['canonical_url'] ?? '');
        $sourceName = self::extractDomain($sourceUrl);

        $domainFields = [];
        foreach (['mining', 'indigenous', 'coforge', 'entertainment', 'rfp', 'recipe', 'job'] as $field) {
            if (isset($data[$field]) && is_array($data[$field])) {
                $domainFields[$field] = $data[$field];
            }
        }
        foreach (['crime_relevance', 'crime_types', 'entertainment_relevance', 'entertainment_categories'] as $field) {
            if (isset($data[$field])) {
                $domainFields[$field] = $data[$field];
            }
        }

        return new self(
            id: (string) ($data['id'] ?? ''),
            title: (string) ($data['title'] ?? ''),
            body: (string) ($data['body'] ?? $data['raw_text'] ?? ''),
            url: (string) ($data['canonical_url'] ?? ''),
            contentType: (string) ($data['content_type'] ?? ''),
            sourceName: $sourceName,
            qualityScore: (int) ($data['quality_score'] ?? 0),
            topics: is_array($data['topics'] ?? null) ? $data['topics'] : [],
            ogImage: (string) ($data['og_image'] ?? ''),
            publishedAt: (string) ($data['published_date'] ?? date('c')),
            metadataJson: $domainFields !== [] ? json_encode($domainFields, JSON_THROW_ON_ERROR) : '{}',
        );
    }

    public function getSearchDocumentId(): string
    {
        return $this->id;
    }

    /** @return array{title: string, body: string} */
    public function toSearchDocument(): array
    {
        return [
            'title' => $this->title,
            'body' => $this->body,
        ];
    }

    /** @return array<string, mixed> */
    public function toSearchMetadata(): array
    {
        return [
            'entity_type' => 'content',
            'content_type' => $this->contentType,
            'source_name' => $this->sourceName,
            'quality_score' => $this->qualityScore,
            'topics' => $this->topics,
            'url' => $this->url,
            'og_image' => $this->ogImage,
            'created_at' => $this->publishedAt,
            'metadata_json' => $this->metadataJson,
        ];
    }

    private static function extractDomain(string $url): string
    {
        if ($url === '') {
            return '';
        }

        $host = parse_url($url, PHP_URL_HOST);
        if (!is_string($host)) {
            return '';
        }

        return preg_replace('/^www\./', '', $host);
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/jones/dev/northcloud-search
vendor/bin/phpunit tests/Document/ContentDocumentTest.php
```

Expected: All 5 tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add composer.json composer.lock src/Document/ContentDocument.php tests/Document/ContentDocumentTest.php
git commit -m "feat: add ContentDocument for mapping Redis messages to FTS5"
```

---

### Task 2: Add Redis subscriber command

**Files:**
- Create: `src/Command/SubscribeCommand.php`
- Modify: `src/Provider/AppServiceProvider.php` (add command registration)

- [ ] **Step 1: Write SubscribeCommand**

Create `src/Command/SubscribeCommand.php`:

```php
<?php

declare(strict_types=1);

namespace App\Command;

use App\Document\ContentDocument;
use Predis\Client as RedisClient;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Input\InputInterface;
use Symfony\Component\Console\Input\InputOption;
use Symfony\Component\Console\Output\OutputInterface;
use Waaseyaa\Search\Fts5\Fts5SearchIndexer;
use Waaseyaa\Search\SearchIndexerInterface;

final class SubscribeCommand extends Command
{
    private int $indexed = 0;
    private int $duplicates = 0;
    private int $errors = 0;

    public function __construct(
        private readonly SearchIndexerInterface $indexer,
    ) {
        parent::__construct();
    }

    protected function configure(): void
    {
        $this->setName('app:subscribe')
            ->setDescription('Subscribe to North Cloud Redis pub/sub and index content into FTS5')
            ->addOption('redis-url', null, InputOption::VALUE_REQUIRED, 'Redis URL', 'tcp://127.0.0.1:6379')
            ->addOption('channels', null, InputOption::VALUE_REQUIRED, 'Channel pattern (comma-separated)', 'content:*');
    }

    protected function execute(InputInterface $input, OutputInterface $output): int
    {
        $redisUrl = $input->getOption('redis-url') ?: getenv('REDIS_URL') ?: 'tcp://127.0.0.1:6379';
        $channelPatterns = explode(',', $input->getOption('channels'));

        if ($this->indexer instanceof Fts5SearchIndexer) {
            $this->indexer->ensureSchema();
        }

        $output->writeln(sprintf('<info>Connecting to Redis at %s...</info>', $redisUrl));
        $output->writeln(sprintf('<info>Subscribing to: %s</info>', implode(', ', $channelPatterns)));

        $redis = new RedisClient($redisUrl);

        $pubsub = $redis->pubSubLoop();
        foreach ($channelPatterns as $pattern) {
            $pubsub->psubscribe(trim($pattern));
        }

        $output->writeln('<info>Listening for content...</info>');

        /** @var object $message */
        foreach ($pubsub as $message) {
            if ($message->kind !== 'pmessage') {
                continue;
            }

            $this->processMessage($message->payload, $message->channel, $output);

            if (($this->indexed + $this->duplicates + $this->errors) % 100 === 0) {
                $output->writeln(sprintf(
                    '<comment>Stats: %d indexed, %d duplicates, %d errors</comment>',
                    $this->indexed,
                    $this->duplicates,
                    $this->errors,
                ));
            }
        }

        return Command::SUCCESS;
    }

    private function processMessage(string $payload, string $channel, OutputInterface $output): void
    {
        $data = json_decode($payload, true);
        if (!is_array($data)) {
            $this->errors++;
            $output->writeln('<error>Invalid JSON received</error>');
            return;
        }

        $id = $data['id'] ?? null;
        if ($id === null) {
            $this->errors++;
            return;
        }

        try {
            $doc = ContentDocument::fromRedisMessage($data);
            $this->indexer->index($doc);
            $this->indexed++;

            $output->writeln(sprintf(
                '  <info>[%s]</info> %s — %s',
                $data['content_type'] ?? 'unknown',
                mb_substr($data['title'] ?? '(no title)', 0, 60),
                $channel,
            ), OutputInterface::VERBOSITY_VERBOSE);
        } catch (\Throwable $e) {
            $this->errors++;
            $output->writeln(sprintf('<error>Index error for %s: %s</error>', $id, $e->getMessage()));
        }
    }
}
```

- [ ] **Step 2: Register the command in AppServiceProvider**

In `src/Provider/AppServiceProvider.php`, add the `commands()` method:

```php
public function commands(
    \Waaseyaa\Entity\EntityTypeManager $entityTypeManager,
    \Waaseyaa\Database\DatabaseInterface $database,
    \Symfony\Contracts\EventDispatcher\EventDispatcherInterface $dispatcher,
): array {
    $indexer = new \Waaseyaa\Search\Fts5\Fts5SearchIndexer($database);

    return [
        new \App\Command\SubscribeCommand($indexer),
    ];
}
```

- [ ] **Step 3: Test the command registers**

```bash
cd /home/jones/dev/northcloud-search
php bin/waaseyaa list | grep app:subscribe
```

Expected: `app:subscribe` appears in the command list.

- [ ] **Step 4: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add src/Command/SubscribeCommand.php src/Provider/AppServiceProvider.php
git commit -m "feat: add Redis subscriber command for FTS5 indexing"
```

---

### Task 3: Update SearchController to use FTS5

**Files:**
- Modify: `src/Controller/SearchController.php`
- Modify: `src/Provider/AppServiceProvider.php` (add FTS5 bindings)

- [ ] **Step 1: Add FTS5 provider binding to AppServiceProvider**

In `src/Provider/AppServiceProvider.php`, update the `register()` method to bind the search provider:

```php
public function register(): void
{
    $this->singleton(\Waaseyaa\Search\SearchProviderInterface::class, function () {
        $database = $this->resolve(\Waaseyaa\Database\DatabaseInterface::class);
        $indexer = new \Waaseyaa\Search\Fts5\Fts5SearchIndexer($database);
        $indexer->ensureSchema();
        return new \Waaseyaa\Search\Fts5\Fts5SearchProvider($database, $indexer);
    });
}
```

Remove the old `NorthCloudClient` singleton.

- [ ] **Step 2: Rewrite SearchController**

Replace `src/Controller/SearchController.php`:

```php
<?php

declare(strict_types=1);

namespace App\Controller;

use Symfony\Component\HttpFoundation\Request;
use Twig\Environment;
use Waaseyaa\Access\AccountInterface;
use Waaseyaa\Search\SearchFilters;
use Waaseyaa\Search\SearchProviderInterface;
use Waaseyaa\Search\SearchRequest;
use Waaseyaa\SSR\SsrResponse;

final class SearchController
{
    public function __construct(
        private readonly Environment $twig,
        private readonly SearchProviderInterface $search,
    ) {}

    public function results(
        array $params,
        array $query,
        AccountInterface $account,
        Request $httpRequest,
    ): SsrResponse {
        $searchQuery = trim($query['q'] ?? '');
        $page = max(1, (int) ($query['page'] ?? 1));
        $contentType = trim($query['type'] ?? '');
        $topic = trim($query['topic'] ?? '');

        $result = null;
        if ($searchQuery !== '') {
            $filters = new SearchFilters(
                topics: $topic !== '' ? [$topic] : [],
                contentType: $contentType,
            );

            $result = $this->search->search(new SearchRequest(
                query: $searchQuery,
                filters: $filters,
                page: $page,
                pageSize: 10,
            ));
        }

        $html = $this->twig->render('search.html.twig', [
            'query' => $searchQuery,
            'result' => $result,
            'activeType' => $contentType,
            'activeTopic' => $topic,
        ]);

        return new SsrResponse(content: $html);
    }
}
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /home/jones/dev/northcloud-search
php -l src/Controller/SearchController.php
php -l src/Provider/AppServiceProvider.php
```

Expected: No syntax errors.

- [ ] **Step 4: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add src/Controller/SearchController.php src/Provider/AppServiceProvider.php
git commit -m "feat: rewrite SearchController to use FTS5 provider"
```

---

### Task 4: Update HomeController to use FTS5

**Files:**
- Modify: `src/Controller/HomeController.php`

- [ ] **Step 1: Rewrite HomeController**

Replace `src/Controller/HomeController.php`:

```php
<?php

declare(strict_types=1);

namespace App\Controller;

use Symfony\Component\HttpFoundation\Request;
use Twig\Environment;
use Waaseyaa\Access\AccountInterface;
use Waaseyaa\Search\SearchFilters;
use Waaseyaa\Search\SearchProviderInterface;
use Waaseyaa\Search\SearchRequest;
use Waaseyaa\SSR\SsrResponse;

final class HomeController
{
    public function __construct(
        private readonly Environment $twig,
        private readonly SearchProviderInterface $search,
    ) {}

    public function index(
        array $params,
        array $query,
        AccountInterface $account,
        Request $httpRequest,
    ): SsrResponse {
        // Fetch recent content — search for wildcard to get latest items
        $recent = $this->search->search(new SearchRequest(
            query: '*',
            filters: new SearchFilters(sortField: 'created_at', sortOrder: 'desc'),
            page: 1,
            pageSize: 12,
        ));

        $html = $this->twig->render('home.html.twig', [
            'recent' => $recent,
        ]);

        return new SsrResponse(content: $html);
    }
}
```

**Note:** The FTS5 provider requires a non-empty query. The `*` query may not work with the escapeQuery method. If it doesn't, we'll need a "browse all" method. But FTS5 doesn't support `*` as wildcard — it's not a valid match-all. We need a different approach for the homepage.

Actually, looking at `Fts5SearchProvider::search()`, it calls `escapeQuery('*')` which strips the `*` and returns `''`, then returns `SearchResult::empty()`. So for the homepage "recent content", we need to query the database directly.

Replace `HomeController` with this instead:

```php
<?php

declare(strict_types=1);

namespace App\Controller;

use Symfony\Component\HttpFoundation\Request;
use Twig\Environment;
use Waaseyaa\Access\AccountInterface;
use Waaseyaa\Database\DatabaseInterface;
use Waaseyaa\SSR\SsrResponse;

final class HomeController
{
    public function __construct(
        private readonly Environment $twig,
        private readonly DatabaseInterface $database,
    ) {}

    public function index(
        array $params,
        array $query,
        AccountInterface $account,
        Request $httpRequest,
    ): SsrResponse {
        $recentItems = $this->fetchRecent(12);
        $typeCounts = $this->fetchTypeCounts();

        $html = $this->twig->render('home.html.twig', [
            'recentItems' => $recentItems,
            'typeCounts' => $typeCounts,
        ]);

        return new SsrResponse(content: $html);
    }

    /** @return list<array<string, mixed>> */
    private function fetchRecent(int $limit): array
    {
        $sql = <<<'SQL'
            SELECT m.document_id, si.title, m.content_type, m.source_name,
                   m.url, m.og_image, m.quality_score, m.topics, m.created_at
            FROM search_metadata m
            JOIN search_index si ON si.document_id = m.document_id
            ORDER BY m.created_at DESC
            LIMIT :limit
        SQL;

        $items = [];
        foreach ($this->database->query($sql, ['limit' => $limit]) as $row) {
            $row['topics'] = json_decode($row['topics'], true) ?: [];
            $items[] = $row;
        }

        return $items;
    }

    /** @return array<string, int> */
    private function fetchTypeCounts(): array
    {
        $sql = <<<'SQL'
            SELECT content_type, COUNT(*) as cnt
            FROM search_metadata
            WHERE content_type != ''
            GROUP BY content_type
            ORDER BY cnt DESC
        SQL;

        $counts = [];
        foreach ($this->database->query($sql, []) as $row) {
            $counts[$row['content_type']] = (int) $row['cnt'];
        }

        return $counts;
    }
}
```

- [ ] **Step 2: Verify syntax**

```bash
cd /home/jones/dev/northcloud-search
php -l src/Controller/HomeController.php
```

Expected: No syntax errors.

- [ ] **Step 3: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add src/Controller/HomeController.php
git commit -m "feat: rewrite HomeController to query FTS5 directly"
```

---

### Task 5: Update SuggestController for FTS5 prefix search

**Files:**
- Modify: `src/Controller/SuggestController.php`

- [ ] **Step 1: Rewrite SuggestController**

The FTS5 provider's `escapeQuery` strips prefix operators, so we query the database directly for autocomplete.

Replace `src/Controller/SuggestController.php`:

```php
<?php

declare(strict_types=1);

namespace App\Controller;

use Symfony\Component\HttpFoundation\Request;
use Waaseyaa\Access\AccountInterface;
use Waaseyaa\Database\DatabaseInterface;
use Waaseyaa\SSR\SsrResponse;

final class SuggestController
{
    public function __construct(
        private readonly DatabaseInterface $database,
    ) {}

    public function suggest(
        array $params,
        array $query,
        AccountInterface $account,
        Request $httpRequest,
    ): SsrResponse {
        $q = trim($query['q'] ?? '');

        if (mb_strlen($q) < 2) {
            return new SsrResponse(
                content: '[]',
                headers: ['Content-Type' => 'application/json'],
            );
        }

        $suggestions = $this->prefixSearch($q, 8);

        return new SsrResponse(
            content: json_encode($suggestions, JSON_THROW_ON_ERROR),
            headers: ['Content-Type' => 'application/json'],
        );
    }

    /** @return list<string> */
    private function prefixSearch(string $prefix, int $limit): array
    {
        // Use FTS5 prefix query directly — quote the term and append *
        $escaped = str_replace('"', '""', $prefix);
        $ftsQuery = '"' . $escaped . '"*';

        $sql = <<<'SQL'
            SELECT DISTINCT si.title
            FROM search_index si
            WHERE search_index MATCH :query
            ORDER BY si.rank
            LIMIT :limit
        SQL;

        $titles = [];
        foreach ($this->database->query($sql, ['query' => $ftsQuery, 'limit' => $limit]) as $row) {
            $titles[] = $row['title'];
        }

        return $titles;
    }
}
```

- [ ] **Step 2: Verify syntax**

```bash
cd /home/jones/dev/northcloud-search
php -l src/Controller/SuggestController.php
```

Expected: No syntax errors.

- [ ] **Step 3: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add src/Controller/SuggestController.php
git commit -m "feat: rewrite SuggestController for FTS5 prefix search"
```

---

### Task 6: Add ContentController for detail view

**Files:**
- Create: `src/Controller/ContentController.php`
- Modify: `src/Provider/AppServiceProvider.php` (add route)
- Create: `templates/content.html.twig`

- [ ] **Step 1: Create ContentController**

Create `src/Controller/ContentController.php`:

```php
<?php

declare(strict_types=1);

namespace App\Controller;

use Symfony\Component\HttpFoundation\Request;
use Twig\Environment;
use Waaseyaa\Access\AccountInterface;
use Waaseyaa\Database\DatabaseInterface;
use Waaseyaa\SSR\SsrResponse;

final class ContentController
{
    public function __construct(
        private readonly Environment $twig,
        private readonly DatabaseInterface $database,
    ) {}

    public function show(
        array $params,
        array $query,
        AccountInterface $account,
        Request $httpRequest,
    ): SsrResponse {
        $id = $params['id'] ?? '';

        $sql = <<<'SQL'
            SELECT m.*, si.title, si.body
            FROM search_metadata m
            JOIN search_index si ON si.document_id = m.document_id
            WHERE m.document_id = :id
        SQL;

        $rows = iterator_to_array($this->database->query($sql, ['id' => $id]));

        if ($rows === []) {
            $html = $this->twig->render('404.html.twig', ['path' => "/content/$id"]);
            return new SsrResponse(content: $html, status: 404);
        }

        $item = $rows[0];
        $item['topics'] = json_decode($item['topics'], true) ?: [];
        $item['metadata'] = json_decode($item['metadata_json'] ?? '{}', true) ?: [];

        $html = $this->twig->render('content.html.twig', ['item' => $item]);

        return new SsrResponse(content: $html);
    }
}
```

- [ ] **Step 2: Add route in AppServiceProvider**

Add to the `routes()` method in `src/Provider/AppServiceProvider.php`:

```php
$router->addRoute('content.show', RouteBuilder::create('/content/{id}')
    ->controller('App\\Controller\\ContentController::show')
    ->methods('GET')
    ->render()
    ->allowAll()
    ->build());
```

- [ ] **Step 3: Create content template**

Create `templates/content.html.twig`:

```twig
{% extends 'base.html.twig' %}

{% block title %}{{ item.title }} — North Cloud{% endblock %}

{% block content %}
<article class="content-detail">
  <header>
    <span class="badge badge-{{ item.content_type }}">{{ item.content_type }}</span>
    <h1>{{ item.title }}</h1>
    <div class="meta">
      <span class="source">{{ item.source_name }}</span>
      <time datetime="{{ item.created_at }}">{{ item.created_at|date('M j, Y') }}</time>
      {% if item.quality_score > 0 %}
        <span class="quality">Quality: {{ item.quality_score }}/100</span>
      {% endif %}
    </div>
  </header>

  {% if item.og_image %}
    <img src="{{ item.og_image }}" alt="{{ item.title }}" class="content-image" loading="lazy">
  {% endif %}

  <div class="content-body">
    {{ item.body|nl2br }}
  </div>

  {% if item.topics is not empty %}
    <div class="topics">
      {% for topic in item.topics %}
        <a href="/search?topic={{ topic }}" class="topic-pill">{{ topic|replace({'_': ' '}) }}</a>
      {% endfor %}
    </div>
  {% endif %}

  {% if item.url %}
    <div class="source-link">
      <a href="{{ item.url }}" target="_blank" rel="noopener">View original source &rarr;</a>
    </div>
  {% endif %}
</article>
{% endblock %}
```

- [ ] **Step 4: Verify syntax**

```bash
cd /home/jones/dev/northcloud-search
php -l src/Controller/ContentController.php
php -l src/Provider/AppServiceProvider.php
```

Expected: No syntax errors.

- [ ] **Step 5: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add src/Controller/ContentController.php src/Provider/AppServiceProvider.php templates/content.html.twig
git commit -m "feat: add content detail page"
```

---

### Task 7: Update templates for FTS5 data structure

**Files:**
- Modify: `templates/base.html.twig`
- Modify: `templates/home.html.twig`
- Modify: `templates/search.html.twig`

The existing templates reference the old ES-proxy data format (`results`, `topicCounts`, etc.). Update them for the new FTS5 `SearchResult` objects.

- [ ] **Step 1: Update search.html.twig**

The SearchController now passes `result` (a `SearchResult` object or null), `query`, `activeType`, `activeTopic`. Rewrite `templates/search.html.twig`:

```twig
{% extends 'base.html.twig' %}

{% block title %}{% if query %}{{ query }} — {% endif %}Search — North Cloud{% endblock %}

{% block content %}
<div class="search-page">
  <form action="/search" method="get" class="search-form">
    <input type="text" name="q" value="{{ query }}" placeholder="Search all content..." autocomplete="off" id="search-input">
    <button type="submit">Search</button>
  </form>

  {% if result %}
    <div class="search-layout">
      {# Facet sidebar #}
      <aside class="facets">
        {% set typeFacet = result.getFacet('content_type') %}
        {% if typeFacet %}
          <div class="facet-group">
            <h3>Content Type</h3>
            {% for bucket in typeFacet.buckets %}
              <a href="/search?q={{ query|url_encode }}&type={{ bucket.key }}&topic={{ activeTopic }}"
                 class="facet-item {% if activeType == bucket.key %}active{% endif %}">
                {{ bucket.key|replace({'_': ' '})|capitalize }} <span class="count">{{ bucket.count }}</span>
              </a>
            {% endfor %}
            {% if activeType %}
              <a href="/search?q={{ query|url_encode }}&topic={{ activeTopic }}" class="facet-clear">Clear filter</a>
            {% endif %}
          </div>
        {% endif %}

        {% set topicFacet = result.getFacet('topics') %}
        {% if topicFacet %}
          <div class="facet-group">
            <h3>Topics</h3>
            {% for bucket in topicFacet.buckets|slice(0, 15) %}
              <a href="/search?q={{ query|url_encode }}&type={{ activeType }}&topic={{ bucket.key }}"
                 class="facet-item {% if activeTopic == bucket.key %}active{% endif %}">
                {{ bucket.key|replace({'_': ' '}) }} <span class="count">{{ bucket.count }}</span>
              </a>
            {% endfor %}
            {% if activeTopic %}
              <a href="/search?q={{ query|url_encode }}&type={{ activeType }}" class="facet-clear">Clear filter</a>
            {% endif %}
          </div>
        {% endif %}
      </aside>

      {# Results #}
      <div class="results">
        <p class="result-count">{{ result.totalHits }} results ({{ result.tookMs }}ms)</p>

        {% for hit in result.hits %}
          <article class="result-card">
            {% if hit.ogImage %}
              <img src="{{ hit.ogImage }}" alt="" class="result-thumb" loading="lazy">
            {% endif %}
            <div class="result-content">
              <h2><a href="/content/{{ hit.id }}">{{ hit.title }}</a></h2>
              {% if hit.highlight %}
                <p class="snippet">{{ hit.highlight|raw }}</p>
              {% endif %}
              <div class="result-meta">
                <span class="badge badge-{{ hit.contentType }}">{{ hit.contentType }}</span>
                <span class="source">{{ hit.sourceName }}</span>
                <time datetime="{{ hit.crawledAt }}">{{ hit.crawledAt|date('M j, Y') }}</time>
              </div>
            </div>
          </article>
        {% endfor %}

        {# Pagination #}
        {% if result.totalPages > 1 %}
          <nav class="pagination">
            {% if result.currentPage > 1 %}
              <a href="/search?q={{ query|url_encode }}&page={{ result.currentPage - 1 }}&type={{ activeType }}&topic={{ activeTopic }}">&laquo; Previous</a>
            {% endif %}
            <span>Page {{ result.currentPage }} of {{ result.totalPages }}</span>
            {% if result.currentPage < result.totalPages %}
              <a href="/search?q={{ query|url_encode }}&page={{ result.currentPage + 1 }}&type={{ activeType }}&topic={{ activeTopic }}">Next &raquo;</a>
            {% endif %}
          </nav>
        {% endif %}
      </div>
    </div>

  {% elseif query %}
    <p class="no-results">No results found for "{{ query }}"</p>
  {% else %}
    <p class="search-hint">Enter a search query above to find content across the North Cloud pipeline.</p>
  {% endif %}
</div>
{% endblock %}
```

- [ ] **Step 2: Update home.html.twig**

Replace `templates/home.html.twig`:

```twig
{% extends 'base.html.twig' %}

{% block title %}North Cloud — Content Pipeline Search{% endblock %}

{% block content %}
<div class="home">
  <section class="hero">
    <h1>North Cloud</h1>
    <p>Search across the entire content pipeline — articles, recipes, jobs, RFPs, and more.</p>
    <form action="/search" method="get" class="search-form search-form-hero">
      <input type="text" name="q" placeholder="Search content..." autocomplete="off" id="search-input">
      <button type="submit">Search</button>
    </form>
  </section>

  {% if typeCounts is not empty %}
    <section class="type-stats">
      <h2>Indexed Content</h2>
      <div class="stat-grid">
        {% for type, count in typeCounts %}
          <a href="/search?q=*&type={{ type }}" class="stat-card">
            <span class="stat-count">{{ count|number_format }}</span>
            <span class="stat-label">{{ type|replace({'_': ' '})|capitalize }}s</span>
          </a>
        {% endfor %}
      </div>
    </section>
  {% endif %}

  {% if recentItems is not empty %}
    <section class="recent">
      <h2>Recently Indexed</h2>
      <div class="card-grid">
        {% for item in recentItems %}
          <article class="content-card">
            {% if item.og_image %}
              <img src="{{ item.og_image }}" alt="" class="card-thumb" loading="lazy">
            {% endif %}
            <div class="card-body">
              <span class="badge badge-{{ item.content_type }}">{{ item.content_type }}</span>
              <h3><a href="/content/{{ item.document_id }}">{{ item.title }}</a></h3>
              <div class="card-meta">
                <span>{{ item.source_name }}</span>
                <time datetime="{{ item.created_at }}">{{ item.created_at|date('M j, Y') }}</time>
              </div>
            </div>
          </article>
        {% endfor %}
      </div>
    </section>
  {% else %}
    <section class="empty-state">
      <p>No content indexed yet. The Redis subscriber will populate search results as content flows through the pipeline.</p>
    </section>
  {% endif %}
</div>
{% endblock %}
```

- [ ] **Step 3: Add CSS for new components to base.html.twig**

Update the `<style>` block in `templates/base.html.twig` to add styles for facets, badges, cards, content detail, and stat grid. Keep the existing base structure (header, footer, autocomplete JS). Add these rules inside the existing `<style>` tag:

```css
/* Badges */
.badge { display: inline-block; padding: 2px 8px; border-radius: 3px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; }
.badge-article { background: #e3f2fd; color: #1565c0; }
.badge-recipe { background: #e8f5e9; color: #2e7d32; }
.badge-job { background: #fff3e0; color: #e65100; }
.badge-rfp { background: #f3e5f5; color: #7b1fa2; }

/* Search layout */
.search-layout { display: grid; grid-template-columns: 220px 1fr; gap: 2rem; margin-top: 1.5rem; }
@media (max-width: 768px) { .search-layout { grid-template-columns: 1fr; } }

/* Facets */
.facet-group { margin-bottom: 1.5rem; }
.facet-group h3 { font-size: 0.85rem; text-transform: uppercase; color: #666; margin-bottom: 0.5rem; }
.facet-item { display: flex; justify-content: space-between; padding: 4px 8px; border-radius: 4px; text-decoration: none; color: #333; font-size: 0.9rem; }
.facet-item:hover, .facet-item.active { background: #e3f2fd; color: #1565c0; }
.facet-item .count { color: #999; font-size: 0.8rem; }
.facet-clear { display: block; font-size: 0.8rem; margin-top: 4px; color: #999; }

/* Result cards */
.result-card { display: flex; gap: 1rem; padding: 1rem 0; border-bottom: 1px solid #eee; }
.result-thumb { width: 120px; height: 80px; object-fit: cover; border-radius: 4px; flex-shrink: 0; }
.result-content h2 { font-size: 1.1rem; margin: 0 0 0.25rem; }
.result-content h2 a { color: #1a0dab; text-decoration: none; }
.result-content h2 a:hover { text-decoration: underline; }
.snippet { font-size: 0.9rem; color: #545454; margin: 0.25rem 0; }
.snippet b { font-weight: 600; color: #333; }
.result-meta { font-size: 0.8rem; color: #999; display: flex; gap: 0.75rem; align-items: center; margin-top: 0.25rem; }
.result-count { color: #666; font-size: 0.9rem; }
.no-results, .search-hint { color: #666; text-align: center; margin-top: 3rem; }

/* Pagination */
.pagination { display: flex; align-items: center; gap: 1rem; justify-content: center; margin-top: 2rem; padding-top: 1rem; border-top: 1px solid #eee; }
.pagination a { color: #1565c0; text-decoration: none; }

/* Homepage */
.hero { text-align: center; padding: 3rem 0; }
.hero h1 { font-size: 2.5rem; margin-bottom: 0.5rem; }
.hero p { color: #666; margin-bottom: 1.5rem; }
.search-form-hero { max-width: 600px; margin: 0 auto; }
.stat-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 1rem; margin-top: 1rem; }
.stat-card { text-align: center; padding: 1.5rem; background: #f8f9fa; border-radius: 8px; text-decoration: none; color: inherit; }
.stat-card:hover { background: #e3f2fd; }
.stat-count { display: block; font-size: 1.5rem; font-weight: 700; }
.stat-label { font-size: 0.85rem; color: #666; }
.card-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1.5rem; margin-top: 1rem; }
.content-card { background: #fff; border: 1px solid #eee; border-radius: 8px; overflow: hidden; }
.card-thumb { width: 100%; height: 160px; object-fit: cover; }
.card-body { padding: 1rem; }
.card-body h3 { font-size: 1rem; margin: 0.5rem 0; }
.card-body h3 a { color: #333; text-decoration: none; }
.card-body h3 a:hover { color: #1565c0; }
.card-meta { font-size: 0.8rem; color: #999; display: flex; gap: 0.5rem; }
.empty-state { text-align: center; color: #999; padding: 3rem; }

/* Content detail */
.content-detail { max-width: 800px; margin: 0 auto; }
.content-detail header { margin-bottom: 1.5rem; }
.content-detail h1 { font-size: 1.8rem; margin: 0.5rem 0; }
.content-detail .meta { font-size: 0.85rem; color: #666; display: flex; gap: 1rem; }
.content-image { width: 100%; max-height: 400px; object-fit: cover; border-radius: 8px; margin-bottom: 1.5rem; }
.content-body { line-height: 1.7; margin-bottom: 2rem; }
.topics { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-bottom: 1.5rem; }
.topic-pill { padding: 4px 12px; background: #f0f0f0; border-radius: 16px; font-size: 0.85rem; text-decoration: none; color: #555; }
.topic-pill:hover { background: #e3f2fd; color: #1565c0; }
.source-link { padding-top: 1rem; border-top: 1px solid #eee; }
.source-link a { color: #1565c0; }
```

- [ ] **Step 4: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add templates/search.html.twig templates/home.html.twig templates/base.html.twig
git commit -m "feat: update templates for FTS5 search results"
```

---

### Task 8: Clean up — remove NorthCloudClient and update .env

**Files:**
- Delete: `src/Support/NorthCloudClient.php`
- Modify: `.env.example`
- Modify: `composer.json` (remove `symfony/http-client` if no longer needed)

- [ ] **Step 1: Delete NorthCloudClient**

```bash
cd /home/jones/dev/northcloud-search
rm src/Support/NorthCloudClient.php
rmdir src/Support 2>/dev/null || true
```

- [ ] **Step 2: Update .env.example**

Replace `.env.example`:

```env
# Redis connection for subscriber
REDIS_URL=tcp://redis:6379

# SQLite database path (shared between web and subscriber)
# DATABASE_PATH=storage/search.sqlite
```

- [ ] **Step 3: Remove symfony/http-client from composer.json**

```bash
cd /home/jones/dev/northcloud-search
composer remove symfony/http-client
```

- [ ] **Step 4: Verify the app still boots**

```bash
cd /home/jones/dev/northcloud-search
php bin/waaseyaa list
```

Expected: No errors, command list shown including `app:subscribe`.

- [ ] **Step 5: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add -A
git commit -m "chore: remove NorthCloudClient, update env for Redis/FTS5"
```

---

### Task 9: Update Dockerfile for web + subscriber

**Files:**
- Modify: `Dockerfile`
- Create: `docker-compose.yml` (local dev)

- [ ] **Step 1: Update Dockerfile**

Replace `Dockerfile`:

```dockerfile
FROM php:8.4-cli-alpine

RUN apk add --no-cache sqlite-dev \
    && docker-php-ext-install pdo_sqlite

COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /app

COPY composer.json composer.lock ./
RUN composer install --no-dev --optimize-autoloader --no-interaction --no-progress

COPY . .

RUN mkdir -p storage && chmod 777 storage

EXPOSE 3003

# Default: run web server. Override CMD for subscriber.
CMD ["php", "-S", "0.0.0.0:3003", "-t", "public"]
```

- [ ] **Step 2: Create docker-compose.yml for local dev**

Create `docker-compose.yml`:

```yaml
services:
  web:
    build: .
    ports:
      - "3003:3003"
    volumes:
      - search-data:/app/storage
    environment:
      - REDIS_URL=tcp://redis:6379
    depends_on:
      - redis

  subscriber:
    build: .
    command: ["php", "bin/waaseyaa", "app:subscribe", "--redis-url=tcp://redis:6379"]
    volumes:
      - search-data:/app/storage
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  search-data:
```

- [ ] **Step 3: Verify Docker build**

```bash
cd /home/jones/dev/northcloud-search
docker build -t northcloud-search:test .
```

Expected: Build completes successfully.

- [ ] **Step 4: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add Dockerfile docker-compose.yml
git commit -m "feat: update Dockerfile, add docker-compose for web + subscriber"
```

---

### Task 10: Add FTS5 metadata_json column to search schema

The Waaseyaa search indexer's `search_metadata` table doesn't have a `metadata_json` column, but our `ContentDocument` returns it in `toSearchMetadata()`. The `Fts5SearchIndexer::index()` method calls `$this->database->insert('search_metadata')->values(...)` with only the known columns. We need to handle this.

**Options:**
1. Store `metadata_json` as part of a different column (e.g., pack into `topics` JSON)
2. Don't store it — omit from metadata, reconstruct from other fields
3. Extend the schema — add column after `ensureSchema()`

**Decision:** Option 2 for now — remove `metadata_json` from `toSearchMetadata()`. The content detail page can show topics, content_type, source etc. from existing metadata columns. Domain-specific fields (mining, indigenous, etc.) are nice-to-have for v2.

**Files:**
- Modify: `src/Document/ContentDocument.php`
- Modify: `tests/Document/ContentDocumentTest.php`

- [ ] **Step 1: Remove metadata_json from ContentDocument**

In `src/Document/ContentDocument.php`, remove the `metadataJson` constructor parameter and the `metadata_json` key from `toSearchMetadata()`:

Remove the `$metadataJson` field from the constructor and `fromRedisMessage()`. Remove the domain field collection code. Remove `'metadata_json' => $this->metadataJson` from `toSearchMetadata()`.

Updated `toSearchMetadata()`:

```php
/** @return array<string, mixed> */
public function toSearchMetadata(): array
{
    return [
        'entity_type' => 'content',
        'content_type' => $this->contentType,
        'source_name' => $this->sourceName,
        'quality_score' => $this->qualityScore,
        'topics' => $this->topics,
        'url' => $this->url,
        'og_image' => $this->ogImage,
        'created_at' => $this->publishedAt,
    ];
}
```

Updated constructor (remove `metadataJson` parameter):

```php
private function __construct(
    private string $id,
    private string $title,
    private string $body,
    private string $url,
    private string $contentType,
    private string $sourceName,
    private int $qualityScore,
    private array $topics,
    private string $ogImage,
    private string $publishedAt,
) {}
```

Updated `fromRedisMessage()` — remove domain field collection, remove `metadataJson` parameter.

- [ ] **Step 2: Update test — remove metadata_json test**

Remove `testFromRedisMessageStoresMetadataJson` from `tests/Document/ContentDocumentTest.php`.

- [ ] **Step 3: Run tests**

```bash
cd /home/jones/dev/northcloud-search
vendor/bin/phpunit tests/Document/ContentDocumentTest.php
```

Expected: All remaining tests pass.

- [ ] **Step 4: Also remove metadata_json from ContentController**

In `src/Controller/ContentController.php`, remove the line:
```php
$item['metadata'] = json_decode($item['metadata_json'] ?? '{}', true) ?: [];
```

And remove the metadata reference from `templates/content.html.twig` if present.

- [ ] **Step 5: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add src/Document/ContentDocument.php tests/Document/ContentDocumentTest.php src/Controller/ContentController.php
git commit -m "fix: remove metadata_json — not in FTS5 schema"
```

---

### Task 11: Finalize AppServiceProvider

After all individual task changes, ensure `AppServiceProvider.php` has the complete final state.

**Files:**
- Modify: `src/Provider/AppServiceProvider.php`

- [ ] **Step 1: Write final AppServiceProvider**

Replace `src/Provider/AppServiceProvider.php`:

```php
<?php

declare(strict_types=1);

namespace App\Provider;

use Waaseyaa\Database\DatabaseInterface;
use Waaseyaa\Foundation\ServiceProvider\ServiceProvider;
use Waaseyaa\Routing\RouteBuilder;
use Waaseyaa\Routing\WaaseyaaRouter;
use Waaseyaa\Search\Fts5\Fts5SearchIndexer;
use Waaseyaa\Search\Fts5\Fts5SearchProvider;
use Waaseyaa\Search\SearchIndexerInterface;
use Waaseyaa\Search\SearchProviderInterface;

final class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->singleton(SearchIndexerInterface::class, function () {
            $database = $this->resolve(DatabaseInterface::class);
            $indexer = new Fts5SearchIndexer($database);
            $indexer->ensureSchema();
            return $indexer;
        });

        $this->singleton(SearchProviderInterface::class, function () {
            $database = $this->resolve(DatabaseInterface::class);
            $indexer = $this->resolve(SearchIndexerInterface::class);
            return new Fts5SearchProvider($database, $indexer);
        });
    }

    public function routes(WaaseyaaRouter $router, ?\Waaseyaa\Entity\EntityTypeManager $entityTypeManager = null): void
    {
        $router->addRoute('home', RouteBuilder::create('/')
            ->controller('App\\Controller\\HomeController::index')
            ->methods('GET')
            ->render()
            ->allowAll()
            ->build());

        $router->addRoute('search', RouteBuilder::create('/search')
            ->controller('App\\Controller\\SearchController::results')
            ->methods('GET')
            ->render()
            ->allowAll()
            ->build());

        $router->addRoute('content.show', RouteBuilder::create('/content/{id}')
            ->controller('App\\Controller\\ContentController::show')
            ->methods('GET')
            ->render()
            ->allowAll()
            ->build());

        $router->addRoute('suggest', RouteBuilder::create('/api/suggest')
            ->controller('App\\Controller\\SuggestController::suggest')
            ->methods('GET')
            ->allowAll()
            ->build());

        $router->addRoute('health', RouteBuilder::create('/health')
            ->controller('App\\Controller\\HealthController::check')
            ->methods('GET')
            ->allowAll()
            ->build());
    }

    public function commands(
        \Waaseyaa\Entity\EntityTypeManager $entityTypeManager,
        \Waaseyaa\Database\DatabaseInterface $database,
        \Symfony\Contracts\EventDispatcher\EventDispatcherInterface $dispatcher,
    ): array {
        $indexer = new Fts5SearchIndexer($database);

        return [
            new \App\Command\SubscribeCommand($indexer),
        ];
    }
}
```

- [ ] **Step 2: Verify everything boots**

```bash
cd /home/jones/dev/northcloud-search
php bin/waaseyaa list
REQUEST_METHOD=GET REQUEST_URI=/ php public/index.php 2>&1 | head -5
```

Expected: Command list shows `app:subscribe`. Homepage renders HTML (may show empty state).

- [ ] **Step 3: Commit**

```bash
cd /home/jones/dev/northcloud-search
git add src/Provider/AppServiceProvider.php
git commit -m "feat: finalize AppServiceProvider with FTS5 bindings and routes"
```

---

### Task 12: Production deployment — Caddy and docker-compose

**Files:**
- Modify: `/home/jones/dev/northcloud-ansible/roles/north-cloud/templates/Caddyfile.j2` (add northcloud.one block)
- Modify: `/home/jones/dev/north-cloud/docker-compose.base.yml` (add northcloud-search service)
- Modify: `/home/jones/dev/north-cloud/docker-compose.prod.yml` (add prod overrides)

- [ ] **Step 1: Add northcloud.one to Caddyfile template**

Read the current Caddyfile template to understand the structure, then add a block for `northcloud.one` that proxies to the internal nginx (port 8443) or directly to the northcloud-search web container (port 3003).

Add to the Caddyfile.j2:

```
northcloud.one {
    reverse_proxy north-cloud-nginx-1:8443 {
        transport http {
            tls_insecure_skip_verify
        }
    }
}
```

Or if bypassing nginx:

```
northcloud.one {
    reverse_proxy north-cloud-northcloud-search-web-1:3003
}
```

The exact approach depends on the current Caddyfile structure — read it first.

- [ ] **Step 2: Add northcloud-search to docker-compose.base.yml**

Add services for web and subscriber:

```yaml
  northcloud-search-web:
    build:
      context: ../northcloud-search
      dockerfile: Dockerfile
    container_name: north-cloud-northcloud-search-web
    ports:
      - "3003:3003"
    volumes:
      - northcloud-search-data:/app/storage
    networks:
      - north-cloud-network
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:3003/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  northcloud-search-subscriber:
    build:
      context: ../northcloud-search
      dockerfile: Dockerfile
    container_name: north-cloud-northcloud-search-subscriber
    command: ["php", "bin/waaseyaa", "app:subscribe", "--redis-url=tcp://redis:6379"]
    volumes:
      - northcloud-search-data:/app/storage
    depends_on:
      - redis
    networks:
      - north-cloud-network
    restart: unless-stopped
```

Add volume:

```yaml
volumes:
  northcloud-search-data:
```

- [ ] **Step 3: Add prod image overrides to docker-compose.prod.yml**

```yaml
  northcloud-search-web:
    image: docker.io/jonesrussell/northcloud-search:latest

  northcloud-search-subscriber:
    image: docker.io/jonesrussell/northcloud-search:latest
```

- [ ] **Step 4: Commit both repos**

```bash
cd /home/jones/dev/northcloud-ansible
git add roles/north-cloud/templates/Caddyfile.j2
git commit -m "feat: add northcloud.one Caddy block"

cd /home/jones/dev/north-cloud
git add docker-compose.base.yml docker-compose.prod.yml
git commit -m "feat: add northcloud-search web + subscriber services"
```

---

### Task 13: CI/CD — GitHub Actions for northcloud-search

**Files:**
- Create: `/home/jones/dev/northcloud-search/.github/workflows/ci.yml`
- Create: `/home/jones/dev/northcloud-search/.github/workflows/deploy.yml`

- [ ] **Step 1: Create CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: shivammathur/setup-php@v2
        with:
          php-version: '8.4'
          extensions: pdo_sqlite
      - run: composer install --no-interaction --prefer-dist
      - run: vendor/bin/phpunit
```

- [ ] **Step 2: Create deploy workflow**

Create `.github/workflows/deploy.yml`:

```yaml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
      - uses: docker/build-push-action@v5
        with:
          push: true
          tags: jonesrussell/northcloud-search:latest
```

- [ ] **Step 3: Commit**

```bash
cd /home/jones/dev/northcloud-search
mkdir -p .github/workflows
git add .github/workflows/ci.yml .github/workflows/deploy.yml
git commit -m "ci: add CI and deploy workflows"
```

---

### Task 14: Smoke test end-to-end locally

- [ ] **Step 1: Start the local stack**

```bash
cd /home/jones/dev/northcloud-search
docker compose up -d --build
```

- [ ] **Step 2: Verify health endpoint**

```bash
curl http://localhost:3003/health
```

Expected: `{"status":"ok"}`

- [ ] **Step 3: Verify homepage renders**

```bash
curl -s http://localhost:3003/ | head -20
```

Expected: HTML with "North Cloud" heading and empty state message.

- [ ] **Step 4: Publish a test message to Redis**

```bash
docker compose exec redis redis-cli PUBLISH content:crime '{"id":"test-1","title":"Test Article About Crime","body":"This is a test article body for smoke testing the search pipeline.","canonical_url":"https://example.com/test","source":"https://example.com","published_date":"2026-03-28T10:00:00Z","quality_score":85,"topics":["crime","local_news"],"content_type":"article","og_image":"","publisher":{"channel":"content:crime","published_at":"2026-03-28T10:00:00Z","route_id":"test"}}'
```

- [ ] **Step 5: Wait a moment, then verify search works**

```bash
sleep 2
curl -s "http://localhost:3003/search?q=crime" | grep -o "Test Article About Crime" || echo "NOT FOUND"
```

Expected: "Test Article About Crime" appears in output.

- [ ] **Step 6: Verify suggest works**

```bash
curl -s "http://localhost:3003/api/suggest?q=test"
```

Expected: JSON array containing "Test Article About Crime".

- [ ] **Step 7: Verify content detail works**

```bash
curl -s "http://localhost:3003/content/test-1" | grep -o "Test Article About Crime" || echo "NOT FOUND"
```

Expected: "Test Article About Crime" appears.

- [ ] **Step 8: Clean up**

```bash
cd /home/jones/dev/northcloud-search
docker compose down
```
