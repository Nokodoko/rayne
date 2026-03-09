# n0ko Portfolio — Style Guide

Derived from `server-room-dark.jpg` hero banner. All values are implementation-ready.

---

## 1. Color Palette

Extracted from the server room corridor image: deep navy blacks, cyan LED strips, crimson accents, steel-blue rack reflections, and white indicator-light pinpoints.

### CSS Custom Properties

```css
:root {
  /* ── Background Scale (dark corridor floors → ceiling) ── */
  --bg-void:        #050810;   /* deepest black — page background */
  --bg-primary:     #0A0E17;   /* dark navy — main surface */
  --bg-secondary:   #0F1420;   /* slightly lifted — cards, nav */
  --bg-tertiary:    #151B2B;   /* steel-dark — elevated panels, chat header */
  --bg-surface:     #1A2332;   /* rack reflection blue — hover states, active surfaces */

  /* ── Cyan Accent (ceiling LED strips) ── */
  --cyan-400:       #00D4FF;   /* primary accent — links, focus rings, headings */
  --cyan-500:       #00B8DB;   /* hover state */
  --cyan-600:       #0097B2;   /* pressed state */
  --cyan-glow:      rgba(0, 212, 255, 0.25);  /* box-shadow glow */
  --cyan-glow-strong: rgba(0, 212, 255, 0.45); /* hero text glow */

  /* ── Red Accent (rack status LEDs) ── */
  --red-500:        #CC2936;   /* danger, error states, sparse highlights */
  --red-600:        #A3212B;   /* hover on red elements */
  --red-glow:       rgba(204, 41, 54, 0.20);

  /* ── Green Signal (status indicators) ── */
  --green-500:      #00FF9F;   /* online dots, success, secondary accent from GitHub profile */
  --green-glow:     rgba(0, 255, 159, 0.20);

  /* ── Text Scale ── */
  --text-primary:   #E8ECF1;   /* high contrast — headings, body */
  --text-secondary: #8B95A8;   /* muted — descriptions, meta */
  --text-muted:     #4E5972;   /* low contrast — timestamps, placeholders */
  --text-on-cyan:   #050810;   /* text on cyan buttons */

  /* ── Borders ── */
  --border-subtle:  #1C2436;   /* card borders, section dividers */
  --border-hover:   #00D4FF33; /* 20% cyan on hover */
  --border-focus:   #00D4FF;   /* focus ring */

  /* ── Shadows ── */
  --shadow-card:    0 4px 24px rgba(0, 0, 0, 0.5);
  --shadow-elevated: 0 8px 40px rgba(0, 0, 0, 0.6);
  --shadow-cyan:    0 0 20px var(--cyan-glow), 0 0 60px rgba(0, 212, 255, 0.10);
}
```

### Usage Rules

| Role | Token | When |
|------|-------|------|
| Page background | `--bg-void` | `<body>`, fullscreen sections |
| Card / panel fill | `--bg-secondary` | Project cards, chat container |
| Elevated surface | `--bg-tertiary` | Nav bar, chat header, footer |
| Primary action | `--cyan-400` | Buttons, links, focus rings |
| Danger / error | `--red-500` | Error messages, destructive actions only |
| Success / online | `--green-500` | Status dots, success toasts |
| Sparse highlight | `--red-500` | Use very sparingly — tag accents, single-pixel lines |

**Ratio target:** ~85% dark backgrounds, ~10% cyan accent, ~3% green signal, ~2% red punctuation.

---

## 2. Typography

Terminal-first aesthetic. JetBrains Mono carries identity; Inter handles readability.

### Font Stack

```css
:root {
  --font-mono:  'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace;
  --font-sans:  'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
```

### Google Fonts Link

```html
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500;600;700&display=swap" rel="stylesheet"/>
```

### Type Scale

| Element | Font | Weight | Size | Letter-spacing | Color |
|---------|------|--------|------|----------------|-------|
| Hero name (`n0ko`) | `--font-mono` | 700 | `clamp(3rem, 8vw, 5rem)` | `-0.04em` | `--cyan-400` with `text-shadow: var(--shadow-cyan)` |
| Hero subtitle | `--font-mono` | 500 | `1.25rem` | `0.02em` | `--text-secondary` |
| Section titles | `--font-mono` | 600 | `1.75rem` | `-0.01em` | `--text-primary` |
| Nav links | `--font-mono` | 500 | `0.9rem` | `0.03em` | `--text-secondary` → `--cyan-400` on hover |
| Body / descriptions | `--font-sans` | 400 | `1rem` | `0` | `--text-secondary` |
| Card titles | `--font-mono` | 600 | `1.15rem` | `0` | `--text-primary` |
| Tags / badges | `--font-mono` | 500 | `0.75rem` | `0.04em` | `--cyan-400` |
| Code blocks | `--font-mono` | 400 | `0.85rem` | `0` | `--text-primary` on `--bg-primary` |
| Chat messages | `--font-sans` | 400 | `0.8rem` | `0` | `--text-primary` |
| Buttons | `--font-mono` | 600 | `0.9rem` | `0.02em` | depends on variant |

### Terminal Prefix Convention

Section headings should use a terminal prompt prefix to reinforce the hacker identity:

```
> whoami          → About section
> ls projects/    → Projects section
> cat contact.md  → Contact section
```

Render the `> ` prefix in `--cyan-400`, the rest in `--text-primary`.

---

## 3. Component Styles

### 3.1 Buttons

**Primary (cyan solid):**
```css
.btn-primary {
  background: var(--cyan-400);
  color: var(--text-on-cyan);
  border: none;
  padding: 0.75rem 1.5rem;
  border-radius: 6px;
  font-family: var(--font-mono);
  font-weight: 600;
  font-size: 0.9rem;
  letter-spacing: 0.02em;
  cursor: pointer;
  transition: background 0.2s ease, box-shadow 0.2s ease, transform 0.15s ease;
  box-shadow: 0 0 16px var(--cyan-glow);
}

.btn-primary:hover {
  background: var(--cyan-500);
  box-shadow: 0 0 24px var(--cyan-glow-strong);
  transform: translateY(-1px);
}
```

**Secondary (ghost/outline):**
```css
.btn-secondary {
  background: transparent;
  color: var(--cyan-400);
  border: 1px solid var(--border-subtle);
  padding: 0.75rem 1.5rem;
  border-radius: 6px;
  font-family: var(--font-mono);
  font-weight: 500;
  font-size: 0.9rem;
  letter-spacing: 0.02em;
  cursor: pointer;
  transition: border-color 0.2s ease, color 0.2s ease;
}

.btn-secondary:hover {
  border-color: var(--cyan-400);
  color: var(--text-primary);
}
```

No gradients on buttons. Flat cyan is cleaner and matches the LED-strip aesthetic. The glow does the work.

### 3.2 Cards (Project Cards)

```css
.project-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 1.75rem;
  transition: border-color 0.25s ease, box-shadow 0.25s ease, transform 0.2s ease;
}

.project-card:hover {
  border-color: var(--cyan-400);
  box-shadow: var(--shadow-card), 0 0 30px var(--cyan-glow);
  transform: translateY(-2px);
}
```

- Card icon: replace unicode with a small inline SVG or a `::before` pseudo-element using `--cyan-400`
- Card title: `--font-mono`, weight 600
- Card description: `--font-sans`, `--text-secondary`
- Tags: pill-shaped, `background: rgba(0, 212, 255, 0.08)`, `color: var(--cyan-400)`, `border: 1px solid rgba(0, 212, 255, 0.15)`

### 3.3 Tags / Badges

```css
.tag {
  display: inline-block;
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  font-weight: 500;
  letter-spacing: 0.04em;
  background: rgba(0, 212, 255, 0.08);
  color: var(--cyan-400);
  border: 1px solid rgba(0, 212, 255, 0.15);
}
```

### 3.4 Navigation

```css
.nav {
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-subtle);
  backdrop-filter: blur(12px);
  position: sticky;
  top: 0;
  z-index: 100;
}

.logo {
  font-family: var(--font-mono);
  font-weight: 700;
  font-size: 1.4rem;
  color: var(--cyan-400);
  text-shadow: 0 0 12px var(--cyan-glow);
  /* No gradient — flat cyan with glow, like an LED readout */
}
```

### 3.5 Chat Widget (Monty)

The chat widget should feel like a terminal window floating over the site.

```css
.chat-container {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  box-shadow: var(--shadow-elevated);
}

.chat-header {
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-subtle);
}

.chat-avatar {
  background: var(--cyan-400);
  color: var(--text-on-cyan);
  font-family: var(--font-mono);
  font-weight: 700;
}

/* User messages: cyan fill */
.chat-message.user .message-content {
  background: var(--cyan-400);
  color: var(--text-on-cyan);
  border-bottom-right-radius: 4px;
}

/* Assistant messages: dark surface with cyan-tinted border */
.chat-message.assistant .message-content {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
  border-bottom-left-radius: 4px;
}

/* Chat toggle button: flat cyan, no gradient */
.chat-toggle {
  background: var(--cyan-400);
  box-shadow: 0 0 20px var(--cyan-glow);
}

.chat-label {
  color: var(--cyan-400);
  font-family: var(--font-mono);
  text-shadow: 0 0 8px var(--cyan-glow);
}

/* Input field */
.chat-input {
  background: var(--bg-primary);
  border: 1px solid var(--border-subtle);
  color: var(--text-primary);
  font-family: var(--font-sans);
}

.chat-input:focus {
  border-color: var(--cyan-400);
  box-shadow: 0 0 8px var(--cyan-glow);
}

/* Send button */
.chat-send {
  background: var(--cyan-400);
}
```

### 3.6 Code Blocks

```css
pre, code {
  font-family: var(--font-mono);
}

code {
  background: var(--bg-primary);
  padding: 0.15rem 0.4rem;
  border-radius: 4px;
  font-size: 0.85em;
  color: var(--cyan-400);
}

pre {
  background: var(--bg-primary);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  padding: 1rem;
  overflow-x: auto;
}

pre code {
  background: none;
  padding: 0;
  color: var(--text-primary);
}
```

---

## 4. Layout & Spacing

### Spacing Scale (8px base)

```css
:root {
  --space-1:  0.25rem;  /*  4px */
  --space-2:  0.5rem;   /*  8px */
  --space-3:  0.75rem;  /* 12px */
  --space-4:  1rem;     /* 16px */
  --space-6:  1.5rem;   /* 24px */
  --space-8:  2rem;     /* 32px */
  --space-12: 3rem;     /* 48px */
  --space-16: 4rem;     /* 64px */
  --space-24: 6rem;     /* 96px */
}
```

### Container

```css
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 var(--space-8);
}

@media (max-width: 767px) {
  .container {
    padding: 0 var(--space-6);
  }
}
```

### Section Rhythm

- Sections: `padding: var(--space-24) 0` (96px vertical)
- Section title → content gap: `var(--space-8)` (32px)
- Card grid gap: `var(--space-6)` (24px)
- Between inline elements: `var(--space-4)` (16px)

### Grid

```css
.projects-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: var(--space-6);
}
```

### Border Radius Scale

| Element | Radius |
|---------|--------|
| Buttons | `6px` |
| Cards | `8px` |
| Chat container | `10px` |
| Tags / pills | `999px` |
| Avatars | `50%` |
| Input fields | `6px` |

Keep radii small. The server room is all hard edges and straight lines. Avoid large rounded corners.

---

## 5. Hero Banner Integration

The hero section is a full-width banner with the server-room-dark.jpg as a background image, overlaid with a dark gradient for text legibility.

### Structure

```html
<section class="hero">
  <div class="hero-backdrop"></div>
  <div class="hero-overlay"></div>
  <div class="hero-content container">
    <h1 class="hero-title">n0ko</h1>
    <p class="hero-subtitle">Systems Hacker & Terminal Enthusiast</p>
    <p class="hero-description">...</p>
    <div class="hero-cta">...</div>
  </div>
</section>
```

### CSS

```css
.hero {
  position: relative;
  min-height: 80vh;
  display: flex;
  align-items: center;
  overflow: hidden;
}

.hero-backdrop {
  position: absolute;
  inset: 0;
  background: url('/static/images/server-room-dark.jpg') center center / cover no-repeat;
  z-index: 0;
}

/* Dark gradient overlay: fade from near-opaque left to transparent right,
   plus a bottom fade into the next section */
.hero-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  background:
    linear-gradient(90deg,
      rgba(5, 8, 16, 0.92) 0%,
      rgba(5, 8, 16, 0.75) 40%,
      rgba(5, 8, 16, 0.45) 70%,
      rgba(5, 8, 16, 0.30) 100%
    ),
    linear-gradient(180deg,
      transparent 60%,
      var(--bg-void) 100%
    );
}

.hero-content {
  position: relative;
  z-index: 2;
  max-width: 640px;
}

.hero-title {
  font-family: var(--font-mono);
  font-weight: 700;
  font-size: clamp(3rem, 8vw, 5rem);
  letter-spacing: -0.04em;
  color: var(--cyan-400);
  text-shadow:
    0 0 20px var(--cyan-glow-strong),
    0 0 60px rgba(0, 212, 255, 0.15);
  margin-bottom: var(--space-4);
}

.hero-subtitle {
  font-family: var(--font-mono);
  font-weight: 500;
  font-size: 1.25rem;
  letter-spacing: 0.02em;
  color: var(--text-secondary);
  margin-bottom: var(--space-6);
}

.hero-description {
  font-family: var(--font-sans);
  font-size: 1.05rem;
  color: var(--text-muted);
  max-width: 480px;
  margin-bottom: var(--space-8);
  line-height: 1.7;
}
```

### Key Decisions

- **No glow orb.** The image itself provides the atmospheric lighting. Remove the `.glow-orb` entirely.
- **Left-aligned text.** Content sits on the left where the overlay is darkest. The server room corridor perspective draws the eye from center-right, creating a natural visual balance.
- **Bottom gradient fade** into `--bg-void` creates a seamless transition to the About section below.
- **Hero subtitle** should use the GitHub identity: "Systems Hacker & Terminal Enthusiast" — not "Software Engineer & Builder".

---

## 6. Section Backgrounds & Transitions

### Background Alternation

Alternate between `--bg-void` and a very subtle shift to avoid visual monotony, using CSS gradients rather than flat colors:

```css
/* Default section */
.section {
  background: var(--bg-void);
  padding: var(--space-24) 0;
  border-top: 1px solid var(--border-subtle);
}

/* Alternate sections get a faint radial glow */
.section:nth-of-type(even) {
  background:
    radial-gradient(ellipse at 20% 50%, rgba(0, 212, 255, 0.03) 0%, transparent 60%),
    var(--bg-void);
}
```

### Section Dividers

Replace hard `border-top` with a subtle gradient line:

```css
.section-divider {
  height: 1px;
  background: linear-gradient(90deg,
    transparent 0%,
    var(--cyan-400) 20%,
    var(--cyan-400) 80%,
    transparent 100%
  );
  opacity: 0.15;
}
```

### Scroll Reveal

Add subtle fade-in-up on scroll for sections (use `IntersectionObserver`, no heavy animation library):

```css
.section[data-reveal] {
  opacity: 0;
  transform: translateY(20px);
  transition: opacity 0.5s ease, transform 0.5s ease;
}

.section[data-reveal].visible {
  opacity: 1;
  transform: translateY(0);
}
```

---

## 7. GitHub Profile Alignment

The portfolio must feel like a direct extension of the GitHub profile at `github.com/Nokodoko`.

### Identity Markers — Must Match

| GitHub Profile | Portfolio |
|----------------|-----------|
| `ROLE="Systems Hacker & Terminal Enthusiast"` | Hero subtitle, exactly |
| `EDITOR="nvim"` / `WM="dwm"` / `OS="Arch btw"` | Mentioned in About section |
| `PHILOSOPHY="Suckless • Unix • KISS"` | Visible in About or footer tagline |
| JetBrains Mono (typing SVG) | Primary heading/code font |
| `#00D4FF` cyan accent | `--cyan-400` — exact match |
| `#00FF9F` green accent | `--green-500` — for status/success only |
| `#0D1117` background | Close to `--bg-primary` (#0A0E17 — slightly deeper to match the image) |
| Terminal prompt headings (`> whoami`) | Section title prefix convention |

### Projects — Must Include

The GitHub profile features these projects. The portfolio should list all of them:

1. **filePick** — TUI file picker (Go) — *currently missing from portfolio*
2. **computeCommander** — CLI compute orchestration (Go) — *currently missing from portfolio*
3. **rayne** — Datadog webhook handler (Go) — present
4. **messages_tui** — Terminal Messages UI (Go) — present
5. **k8s_the_hard_way** — present (but not on GitHub profile featured)
6. **Monty** — local AI agent — present

### Tagline

Footer or hero area should include: `"Built with caffeine and spite"` — matches the GitHub profile footer.

### Connect Links

Add `n0kos.com` and LinkedIn (`cmonty614`) links, matching the GitHub profile's connect section.

---

## 8. Interaction & Animation Guidelines

### Principles

- **Subtle.** No flashy transitions. The server room is static, quiet, and deliberate.
- **Functional.** Animations communicate state changes, not decoration.
- **Fast.** `0.15s–0.3s` for micro-interactions. Nothing over `0.5s`.

### Specific

| Interaction | Effect |
|-------------|--------|
| Button hover | `translateY(-1px)`, cyan glow intensifies |
| Card hover | `translateY(-2px)`, border goes cyan, faint cyan box-shadow |
| Link hover | Color shifts from `--text-secondary` to `--cyan-400`, `0.2s` |
| Nav link active | Bottom border `2px solid --cyan-400` |
| Chat open/close | `opacity` + `translateY(8px)`, `0.25s ease` |
| Focus ring | `outline: 2px solid var(--cyan-400); outline-offset: 2px;` |
| Scroll reveal | `opacity` + `translateY(20px)`, `0.5s ease`, triggered by IntersectionObserver |

### Cursor

Consider `cursor: url('/static/images/cursor-cyan.svg'), auto` for personality, but only if performance is verified. Optional.

---

## 9. Responsive Breakpoints

```css
/* Mobile-first base styles, then: */
@media (min-width: 768px)  { /* Tablet  */ }
@media (min-width: 1024px) { /* Desktop */ }
@media (min-width: 1441px) { /* Wide    */ }
```

### Mobile Considerations

- Hero image uses `background-position: 65% center` to keep the corridor lighting visible
- Hero overlay increases opacity on left to maintain text contrast on small screens
- Nav collapses to a hamburger or scrollable row
- Chat widget: full-width minus margins, `max-height: 50vh`
- Project grid: single column

---

## 10. File Organization

```
frontend/
├── static/
│   ├── css/
│   │   └── style.css          ← single stylesheet, all custom properties at top
│   ├── images/
│   │   └── server-room-dark.jpg  ← hero banner
│   └── js/
│       └── chat.js
├── templates/
│   ├── layout.templ
│   ├── index.templ
│   └── chat.templ
└── STYLE_GUIDE.md             ← this file
```

Keep it as a single CSS file. No preprocessor, no CSS-in-JS, no Tailwind. Suckless philosophy: one file, readable, auditable.

---

## Summary of Changes from Current Site

| Aspect | Current | New |
|--------|---------|-----|
| Primary accent | `#8b5cf6` purple | `#00D4FF` cyan |
| Secondary accent | `#6366f1` indigo | `#00FF9F` green (sparse) |
| Background | `#0a0a0b` neutral black | `#050810` deep navy-black |
| Headings font | Inter | JetBrains Mono |
| Hero visual | Purple glow orb | Server room photo with overlay |
| Button style | Purple gradient | Flat cyan with glow shadow |
| Identity | "Software Engineer & Builder" | "Systems Hacker & Terminal Enthusiast" |
| Section headings | Plain text | Terminal prompt prefix (`> command`) |
| Card borders on hover | Purple | Cyan with glow |
| Featured projects | 4 projects | 6 projects (add filePick, computeCommander) |
| Gradient usage | Frequent (buttons, logo, text) | Minimal — flat colors with glow effects |
