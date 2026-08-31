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
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "clamp(3.75rem, 15vw, 5.5rem)"
    fontWeight: 750
    lineHeight: 0.9
  body:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "1rem"
    fontWeight: 650
    lineHeight: 1.25
  control:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 750
    lineHeight: 1.2
  note:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "1.05rem"
    fontWeight: 650
    lineHeight: 1.35
  meta:
    fontFamily: "system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 650
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
components:
  button:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.square}"
    padding: "0.875rem 1rem"
---

# Design System: Pool

## Overview

**Creative North Star: "The Shared Clipboard"**

Pool is a single warm sheet with the blunt clarity of a physical utility label. Its hierarchy comes from scale, weight, and hard black rules rather than decoration. The interface stays dense enough for frequent operation while leaving the note as the dominant working area.

## Colors

Warm paper is the field; nearly black ink owns structure and action; muted ink supports metadata; red appears only for actionable failures.

## Typography

System UI typography keeps the tool immediate and dependency-free. The display is oversized and tightly set; note text is 1.05rem, controls use 1.25rem, metadata uses 0.875rem, and action glyphs range from 1.25rem to 1.75rem.

## Layout

A single narrow column uses 2rem side gutters on mobile and caps at 36rem. Files precede the note, controls align to a two-column grid, and the editor grows with the viewport. Desktop preserves the same object-like composition rather than becoming a dashboard.

## Elevation & Depth

The system is completely flat. Borders, spacing, and filled interaction states communicate hierarchy; shadows are not used.

## Shapes

All interactive and content surfaces are square, with 3–4px ink borders. Dashed borders indicate file-drop affordances.

## Components

Buttons are wide, bold, and sentence case. Focus and hover invert ink and paper. File rows use filename and size at left with direct actions at right. Global errors are plain red text above the affected content.

## Do's and Don'ts

### Do:

- **Do** keep actions explicit and keyboard reachable.
- **Do** use the red error voice only for failures with a recovery path.
- **Do** preserve the single-column mobile-first reading order.

### Don't:

- **Don't** add shadows, gradients, rounded cards, or decorative icons.
- **Don't** introduce a second accent color.
- **Don't** hide upload state or errors behind transient toasts.
