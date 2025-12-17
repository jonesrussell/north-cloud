# Streetcode UI Theme

A modern Drupal 11 theme built with Tailwind CSS 4, featuring a clean, responsive design system.

## Features

- **Tailwind CSS 4**: Modern utility-first CSS framework
- **Responsive Design**: Mobile-first approach with breakpoints
- **Accessibility**: WCAG compliant with proper ARIA labels and keyboard navigation
- **Modern UI Components**: Styled buttons, forms, menus, breadcrumbs, and more
- **Custom Color Palette**: Primary and accent color schemes
- **Typography**: Clean, readable font system

## Setup

### Prerequisites

- Node.js 18+ and npm
- Drupal 11

### Installation

1. Install dependencies:
```bash
cd web/themes/custom/streetcode_ui
npm install
```

2. Build Tailwind CSS:
```bash
npm run build
```

For development with watch mode:
```bash
npm run dev
```

**Note**: Tailwind CSS 4 uses PostCSS for processing. The `postcss.config.js` file is included and will be used automatically by the Tailwind CLI.

## Development

### Building CSS

The theme uses Tailwind CSS 4, which needs to be compiled before use:

```bash
# Production build (minified)
npm run build

# Development build with watch mode
npm run dev
```

The compiled CSS is output to `css/tailwind.min.css` and is automatically included via the theme's library system.

### File Structure

```
streetcode_ui/
├── css/
│   ├── tailwind.css          # Tailwind source file
│   ├── tailwind.min.css      # Compiled CSS (generated)
│   └── components/           # Component-specific styles
├── templates/                # Twig templates
├── package.json              # Node.js dependencies
├── tailwind.config.js        # Tailwind configuration
└── streetcode_ui.libraries.yml # Drupal library definitions
```

### Customization

#### Colors

Edit `tailwind.config.js` to customize the color palette:

```javascript
colors: {
  primary: {
    // Your primary color scale
  },
  accent: {
    // Your accent color scale
  },
}
```

#### Components

Component styles are defined in `css/tailwind.css` using Tailwind's `@layer components` directive. You can add custom component classes there.

#### Templates

Templates are located in `templates/` and use Tailwind utility classes. Key templates:

- `layout/page.html.twig` - Main page layout
- `layout/html.html.twig` - HTML structure
- `block/block--system-branding-block.html.twig` - Site branding
- `navigation/menu.html.twig` - Navigation menus
- `content/node.html.twig` - Node display
- `navigation/breadcrumb.html.twig` - Breadcrumb navigation
- `misc/status-messages.html.twig` - Status messages

## Design System

### Colors

- **Primary**: Blue scale (used for links, buttons, active states)
- **Accent**: Purple scale (used for highlights)
- **Gray**: Neutral scale (used for text, borders, backgrounds)

### Typography

- **Font Family**: Inter (system fallback)
- **Headings**: Bold, larger sizes
- **Body**: Regular weight, readable line-height

### Components

All components follow modern design principles:
- Rounded corners
- Subtle shadows
- Smooth transitions
- Hover states
- Focus states for accessibility

## Browser Support

- Modern browsers (Chrome, Firefox, Safari, Edge)
- IE11 not supported (Drupal 11 requirement)

## Additional Resources

- [Tailwind CSS Documentation](https://tailwindcss.com/docs)
- [Drupal Theme Development](https://www.drupal.org/docs/theming-drupal)
- [Starterkit Theme Documentation](https://www.drupal.org/docs/core-modules-and-themes/core-themes/starterkit-theme)
