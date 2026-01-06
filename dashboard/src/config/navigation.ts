import {
  Activity,
  Download,
  ListTodo,
  Link,
  Filter,
  Calendar,
  Globe,
  MapPin,
  Star,
  Brain,
  BarChart3,
  Database,
  Share2,
  GitBranch,
  Radio,
  FileText,
  Rss,
  ScrollText,
  Settings,
  HeartPulse,
  Shield,
  HardDrive,
  type LucideIcon,
} from 'lucide-vue-next'

export interface NavItem {
  title: string
  path: string
  icon: LucideIcon
}

export interface NavSection {
  title: string
  icon: LucideIcon
  path?: string
  quickAction?: {
    label: string
    path: string
  }
  children?: NavItem[]
}

export const navigation: NavSection[] = [
  {
    title: 'Pipeline Monitor',
    icon: Activity,
    path: '/',
  },
  {
    title: 'Content Intake',
    icon: Download,
    quickAction: { label: 'New Job', path: '/intake/jobs/new' },
    children: [
      { title: 'Jobs', path: '/intake/jobs', icon: ListTodo },
      { title: 'Queued Links', path: '/intake/queued-links', icon: Link },
      { title: 'Rules', path: '/intake/rules', icon: Filter },
    ],
  },
  {
    title: 'Source Scheduling',
    icon: Calendar,
    quickAction: { label: 'Add Source', path: '/scheduling/sources/new' },
    children: [
      { title: 'Sources', path: '/scheduling/sources', icon: Globe },
      { title: 'Cities', path: '/scheduling/cities', icon: MapPin },
      { title: 'Reputation', path: '/scheduling/reputation', icon: Star },
    ],
  },
  {
    title: 'Content Intelligence',
    icon: Brain,
    quickAction: { label: 'View Stats', path: '/intelligence/stats' },
    children: [
      { title: 'Classifier Stats', path: '/intelligence/stats', icon: BarChart3 },
      { title: 'Indexes', path: '/intelligence/indexes', icon: Database },
    ],
  },
  {
    title: 'Distribution Engine',
    icon: Share2,
    quickAction: { label: 'New Route', path: '/distribution/routes/new' },
    children: [
      { title: 'Routes', path: '/distribution/routes', icon: GitBranch },
      { title: 'Channels', path: '/distribution/channels', icon: Radio },
      { title: 'Articles', path: '/distribution/articles', icon: FileText },
    ],
  },
  {
    title: 'External Feeds',
    icon: Rss,
    children: [
      { title: 'Redis Streams', path: '/feeds/streams', icon: Activity },
      { title: 'Delivery Logs', path: '/feeds/logs', icon: ScrollText },
    ],
  },
  {
    title: 'System Overview',
    icon: Settings,
    children: [
      { title: 'Health', path: '/system/health', icon: HeartPulse },
      { title: 'Auth', path: '/system/auth', icon: Shield },
      { title: 'Cache', path: '/system/cache', icon: HardDrive },
    ],
  },
]

// Helper to find the current section based on route path
export function getCurrentSection(path: string): NavSection | undefined {
  // Check exact matches first
  const exactMatch = navigation.find((section) => section.path === path)
  if (exactMatch) return exactMatch

  // Check children
  for (const section of navigation) {
    if (section.children) {
      const childMatch = section.children.find((child) => path.startsWith(child.path))
      if (childMatch) return section
    }
  }

  return undefined
}

// Helper to get breadcrumb items for a path
export function getBreadcrumbs(path: string): { label: string; path: string }[] {
  const breadcrumbs: { label: string; path: string }[] = []

  // Always add home
  breadcrumbs.push({ label: 'Pipeline Monitor', path: '/' })

  if (path === '/') return breadcrumbs

  // Find section and child
  for (const section of navigation) {
    if (section.path === path) {
      breadcrumbs.push({ label: section.title, path: section.path })
      return breadcrumbs
    }

    if (section.children) {
      for (const child of section.children) {
        if (path === child.path || path.startsWith(child.path + '/')) {
          breadcrumbs.push({ label: section.title, path: section.children[0].path })
          breadcrumbs.push({ label: child.title, path: child.path })
          return breadcrumbs
        }
      }
    }
  }

  return breadcrumbs
}
