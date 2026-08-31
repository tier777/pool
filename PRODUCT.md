# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Pool is a private, single-user utility for quickly moving short notes and files between the owner's devices.

## Product Purpose

Pool keeps one shared note and a small collection of downloadable files in one self-hosted place. Each host serves exactly one Pool, protected by one password. Success means the latest text and uploaded files are immediately available after authentication.

## Operating Context

The product runs as one Go process backed by one SQLite database and a self-contained browser client. It is used on mobile and desktop browsers, commonly behind a local HTTPS reverse proxy.

## Capabilities and Constraints

- Password-gated, eight-hour server-side sessions.
- No pool switcher or logout control; session expiry returns the user to the password gate.
- One autosaved shared note.
- Authenticated streaming file upload, download, and deletion, bounded by the host filesystem and reverse-proxy configuration.
- No third-party browser runtime or separate object store.

## Brand Commitments

The product name is Pool. The supplied issue #13 mockups establish a concise, warm, high-contrast visual identity.

## Evidence on Hand

Issue #13 contains four mobile reference mockups for the base screen, global error line, file list, add-files control, and drag state. No commercial claims or external brand assets exist.

## Product Principles

- Keep the shortest path between opening Pool and sharing content.
- Make current state and failures unmistakable.
- Keep deployment and backup self-contained.
- Preserve keyboard and assistive-technology access.
