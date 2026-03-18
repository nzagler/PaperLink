# HomeExplorer Context Menu & Rename Implementation Plan

## Checklist

1. Analyze existing item actions and determine how to replace the per-card three-dot dropdown with a right-click context menu that uses the shadcn dropdown components.
2. Design state management for the context menu (open state, targeted item, pointer position) and for the rename dialog (open state, form input and validation, loading/error handling).
3. Implement context menu behavior within `web/src/components/own/home/HomeExplorer.vue`, ensuring right-click opens the dropdown and left-click navigation remains.
4. Add UI for rename functionality (dialog with input) and wire it to API calls using `/api/v1/directory/update/:id` for folders and `/api/v1/document/update` (or whichever endpoint exists) for documents.
5. Wire delete action into the context menu, remove the old trigger button, and ensure tree state updates after rename/delete.
6. Test interactions locally (right-click, rename, delete) and document next steps or caveats.

## Notes

- Directory update endpoint exists as `PATCH /api/v1/directory/update/:id`. Document update currently is `POST /api/v1/document/update` requiring JSON body with `uuid`. Need to set payload accordingly based on item type (likely use item ID for file UUID).
- Use existing design tokens (emerald palette) for focus rings and context menu styling; maintain parity with other actions.
- Remember to prevent default context menu with `@contextmenu.prevent` and to stop propagation to avoid navigation when right-clicking.
- Add inline comments sparingly to explain non-obvious logic, like why we store context menu coordinates.
- After rename success, update the in-memory tree or reload via `loadTree()` to ensure consistent state.

