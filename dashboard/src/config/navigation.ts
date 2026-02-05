import {
  Activity,
  FileText,
  AlertTriangle,
  Brain,
  MapPin,
  Pickaxe,
  Database,
  Download,
  ListTodo,
  Link,
  Filter,
  Globe,
  Building2,
  Star,
  Radio,
  GitBranch,
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
  // Operations - daily cockpit
  {
    title: 'Operations',
    icon: Activity,
    children: [
      { title: 'Pipeline Monitor', path: '/', icon: Activity },
      { title: 'Recent Articles', path: '/operations/articles', icon: FileText },
      { title: 'Review Queue', path: '/operations/review', icon: AlertTriangle },
    ],
  },
  // Intelligence - new value from Option B
  {
    title: 'Intelligence',
    icon: Brain,
    quickAction: { label: 'View Stats', path: '/intelligence/crime' },
    children: [
      { title: 'Crime Breakdown', path: '/intelligence/crime', icon: AlertTriangle },
      { title: 'Mining Breakdown', path: '/intelligence/mining', icon: Pickaxe },
      { title: 'Location Breakdown', path: '/intelligence/location', icon: MapPin },
      { title: 'Index Explorer', path: '/intelligence/indexes', icon: Database },
    ],
  },
  // Content Intake - fix upstream issues
  {
    title: 'Content Intake',
    icon: Download,
    quickAction: { label: 'New Job', path: '/intake/jobs?create=true' },
    children: [
      { title: 'Crawler Jobs', path: '/intake/jobs', icon: ListTodo },
      { title: 'Discovered Links', path: '/intake/discovered-links', icon: Link },
      { title: 'Rules', path: '/intake/rules', icon: Filter },
    ],
  },
  // Sources - manage the ecosystem
  {
    title: 'Sources',
    icon: Globe,
    quickAction: { label: 'Add Source', path: '/sources/new' },
    children: [
      { title: 'All Sources', path: '/sources', icon: Globe },
      { title: 'Cities', path: '/sources/cities', icon: Building2 },
      { title: 'Reputation', path: '/sources/reputation', icon: Star },
    ],
  },
  // Distribution - where content goes
  {
    title: 'Distribution',
    icon: Radio,
    quickAction: { label: 'New Route', path: '/distribution/routes/new' },
    children: [
      { title: 'Channels', path: '/distribution/channels', icon: Radio },
      { title: 'Routes', path: '/distribution/routes', icon: GitBranch },
      { title: 'Delivery Logs', path: '/distribution/logs', icon: ScrollText },
    ],
  },
  // System - rarely used but essential
  {
    title: 'System',
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
  for (const section of navigation) {
    if (section.path === path) return section
    if (section.children) {
      const childMatch = section.children.find(
        (child) => path === child.path || path.startsWith(child.path + '/')
      )
      if (childMatch) return section
    }
  }
  return undefined
}

// Helper to get breadcrumb items for a path
export function getBreadcrumbs(path: string): { label: string; path: string }[] {
  const breadcrumbs: { label: string; path: string }[] = []

  for (const section of navigation) {
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
