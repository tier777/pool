---
name: Pool
description: A compact, tactile shared note and file surface.
colors:
  paper: "#efd09e"
  ink: "#272727"
  muted-ink: "#66583f"
  error: "#c92f24"
typography:
  display:
    fontFamily: "-apple-system, BlinkMacSystemFont, SF Pro Text, Segoe UI, sans-serif"
    fontSize: "4rem"
    fontWeight: 700
    lineHeight: 1.1875
  body:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "1rem"
    fontWeight: 700
    lineHeight: 1.2
  control:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 700
    lineHeight: 1.2
  note:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "1rem"
    fontWeight: 700
    lineHeight: 1.2
  meta:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 700
    lineHeight: 1.25
  icon:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "1.75rem"
    fontWeight: 650
    lineHeight: 1
rounded:
  square: "0"
spacing:
  xs: "0.5rem"
  sm: "1rem"
  md: "1.5rem"
  lg: "2rem"
components:
  button:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.square}"
    padding: "0.75rem 1rem"
---

# Design System: Pool

## Overview

**Creative North Star: "The Shared Clipboard"**

Pool is a single warm sheet with the blunt clarity of a physical utility label. Its hierarchy comes from scale, weight, and hard black rules rather than decoration. The interface stays dense enough for frequent operation while leaving the note as the dominant working area.

## Colors

Warm paper is the field; nearly black ink owns structure and action; muted ink supports metadata; red appears only for actionable failures.

## Typography

System UI typography keeps the tool immediate and dependency-free. The 64px display, 16px note text, 24px controls, and 14px metadata follow the named Figma frames.

## Layout

A single 393px composition uses 32px side gutters, a fixed 256px editor, and the 8/16/24/32 spacing scale. The title and error line precede the note, controls align to a two-column grid, and files or the add-files target follow the actions. Desktop preserves the same object-like composition rather than becoming a dashboard.

## Elevation & Depth

The system is completely flat. Borders, spacing, and filled interaction states communicate hierarchy; shadows are not used.

## Shapes

All interactive and content surfaces are square, with 3–4px ink borders. Dashed borders indicate file-drop affordances.

## Components

Buttons are wide, bold, and sentence case. Button focus and hover invert ink and paper; text fields retain their single static border without an added focus outline or fill. File rows use filename and size at left with direct actions at right. Global errors occupy the reserved line below the title. Saving, loading, and copied states are not displayed.

## Do's and Don'ts

### Do:

- **Do** keep actions explicit and keyboard reachable.
- **Do** use the red error voice only for failures with a recovery path.
- **Do** preserve the single-column mobile-first reading order.

### Don't:

- **Don't** add shadows, gradients, rounded cards, or decorative icons.
- **Don't** introduce a second accent color.
- **Don't** hide upload state or errors behind transient toasts.
