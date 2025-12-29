import type { Component } from 'vue'
import {
  HomeIcon,
  ChartBarIcon,
  BriefcaseIcon,
  DocumentTextIcon,
  FunnelIcon,
  MegaphoneIcon,
  FolderIcon,
  MagnifyingGlassIcon,
  NewspaperIcon,
  StarIcon,
  MapPinIcon,
  LinkIcon,
} from '@heroicons/vue/24/outline'

export interface NavigationItem {
  label: string
  path: string
  icon: Component
  description?: string
  exact?: boolean // Use exact path matching
  external?: boolean // External link (opens in new tab)
}

export interface NavigationSection {
  id: string
  label: string
  icon: Component
  description: string
  items: NavigationItem[]
  order: number
}

/**
 * Navigation configuration organized by content processing pipeline:
 * 1. Dashboard (Overview)
 * 2. Crawler (Content Acquisition)
 * 3. Classifier (Content Enrichment)
 * 4. Publisher (Content Distribution)
 * 5. Sources (Source Management)
 * 6. Analytics (Consolidated Statistics)
 */
export const navigationSections: NavigationSection[] = [
  {
    id: 'dashboard',
    label: 'Dashboard',
    icon: HomeIcon,
    description: 'System overview and health status',
    order: 1,
    items: [
      {
        label: 'Dashboard',
        path: '/',
        icon: HomeIcon,
        description: 'Main dashboard with system overview',
        exact: true,
      },
      {
        label: 'Search',
        path: import.meta.env.DEV ? 'http://localhost:3003/' : '/',
        icon: MagnifyingGlassIcon,
        description: 'Search across all content',
        external: true,
      },
    ],
  },
  {
    id: 'crawler',
    label: 'Crawler',
    icon: FunnelIcon,
    description: 'Web crawler for content acquisition from news sources',
    order: 2,
    items: [
      {
        label: 'Jobs',
        path: '/crawler/jobs',
        icon: BriefcaseIcon,
        description: 'Manage crawl jobs and schedules',
      },
      {
        label: 'Queued Links',
        path: '/crawler/queued-links',
        icon: LinkIcon,
        description: 'View and manage discovered links',
      },
    ],
  },
  {
    id: 'classifier',
    label: 'Classifier',
    icon: FunnelIcon,
    description: 'Content classification and quality scoring engine',
    order: 3,
    items: [
      {
        label: 'Rules',
        path: '/classifier/rules',
        icon: DocumentTextIcon,
        description: 'Classification rules and patterns',
      },
      {
        label: 'Source Reputation',
        path: '/classifier/sources',
        icon: StarIcon,
        description: 'Track and manage source quality scores',
      },
    ],
  },
  {
    id: 'publisher',
    label: 'Publisher',
    icon: MegaphoneIcon,
    description: 'Content publishing and distribution hub',
    order: 4,
    items: [
      {
        label: 'Overview',
        path: '/publisher',
        icon: HomeIcon,
        description: 'Publisher dashboard and quick stats',
        exact: true,
      },
      {
        label: 'Sources',
        path: '/publisher/sources',
        icon: FolderIcon,
        description: 'Configure Elasticsearch sources to monitor',
      },
      {
        label: 'Channels',
        path: '/publisher/channels',
        icon: MegaphoneIcon,
        description: 'Manage Redis pub/sub channels',
      },
      {
        label: 'Routes',
        path: '/publisher/routes',
        icon: DocumentTextIcon,
        description: 'Define source-to-channel routing rules',
      },
      {
        label: 'Recent Articles',
        path: '/publisher/articles',
        icon: NewspaperIcon,
        description: 'View recently published articles',
      },
    ],
  },
  {
    id: 'sources',
    label: 'Sources',
    icon: FolderIcon,
    description: 'Manage news sources and cities',
    order: 5,
    items: [
      {
        label: 'Manage Sources',
        path: '/sources',
        icon: FolderIcon,
        description: 'Add, edit, and remove news sources',
        exact: true,
      },
      {
        label: 'Cities',
        path: '/sources/cities',
        icon: MapPinIcon,
        description: 'Manage city configurations',
      },
    ],
  },
  {
    id: 'analytics',
    label: 'Analytics',
    icon: ChartBarIcon,
    description: 'Consolidated statistics and metrics across all services',
    order: 6,
    items: [
      {
        label: 'System Analytics',
        path: '/analytics',
        icon: ChartBarIcon,
        description: 'View statistics for Crawler, Classifier, and Publisher',
        exact: true,
      },
    ],
  },
]

/**
 * Get navigation sections sorted by order
 */
export function getNavigationSections(): NavigationSection[] {
  return [...navigationSections].sort((a, b) => a.order - b.order)
}

/**
 * Get all navigation items (flattened) for search
 */
export function getAllNavigationItems(): NavigationItem[] {
  return navigationSections.flatMap((section) => section.items)
}

/**
 * Find navigation item by path
 */
export function findNavigationItemByPath(path: string): NavigationItem | undefined {
  return getAllNavigationItems().find((item) => item.path === path)
}

/**
 * Get section by ID
 */
export function getSectionById(id: string): NavigationSection | undefined {
  return navigationSections.find((section) => section.id === id)
}
