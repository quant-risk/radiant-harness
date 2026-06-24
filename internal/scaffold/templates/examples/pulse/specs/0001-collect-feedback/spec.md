---
name: spec
description: Spec — Collect feedback (golden example). Active while feature is in progress.
alwaysApply: true
---

# Spec — Collect feedback

## Summary
The widget sends a feedback (text + context) and the system stores it, returning an id.

## Acceptance criteria

### AC-1: valid feedback is accepted
- **Given** non-empty text and a context
- **When** the widget sends
- **Then** the feedback is stored and returns an `id`

### AC-2: empty feedback is rejected
- **Given** empty text (or only spaces)
- **When** sending
- **Then** returns validation error and does NOT store

### AC-3: feedback above max size is rejected
- **Given** text with more than 1000 characters
- **When** sending
- **Then** returns validation error

## Out of scope
- Moderation / anti-spam.
