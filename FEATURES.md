# dbus-debug Feature Ideas

`dbus-debug` aims to be an interactive TUI for exploring, debugging, and interacting with D-Bus without manually crafting `busctl` or `gdbus` commands.

## Core Explorer

- Select between session bus and system bus.
- List all bus names/services on the selected bus.
- Distinguish well-known names from unique names like `:1.42`.
- Browse object paths recursively from introspection data.
- Browse interfaces on each object path.
- Browse methods, properties, and signals for each interface.
- Show a yazi-like stack of panes for navigation context.
- Preserve cursor/scroll position per pane.
- Support keyboard-first navigation.

## Details Pane

- Show metadata for the currently selected item.
- For bus names:
  - owner unique name
  - activatable status
  - process/user info where available
- For object paths:
  - full object path
  - child node count
  - interface count
- For interfaces:
  - methods/properties/signals summary
- For methods:
  - input args
  - output args
  - full signature
  - annotations
- For properties:
  - type
  - access mode: read/write/readwrite
  - current value
- For signals:
  - emitted args
  - signature
  - match/subscription status

## Method Calls

- Call methods directly from the TUI.
- Support no-argument method calls first.
- Generate an argument input form from introspection signatures.
- Parse D-Bus primitive types:
  - string
  - boolean
  - integer types
  - double
  - object path
  - signature
- Parse container types:
  - arrays
  - structs
  - dictionaries
  - variants
- Show method result values in a readable tree/table view.
- Show D-Bus errors clearly with name and message.
- Keep method call history.
- Allow rerunning previous calls.
- Allow copying equivalent `busctl call` command.

## Properties

- Read property values via `org.freedesktop.DBus.Properties.Get`.
- Read all properties via `GetAll`.
- Refresh property values manually.
- Auto-refresh selected/readable properties.
- Edit writable properties via `Properties.Set`.
- Show property changes from `PropertiesChanged` signals.
- Copy equivalent `busctl get-property` / `set-property` command.

## Signals

- Subscribe/unsubscribe to selected signals.
- Show live signal events in an event log pane.
- Support filtering by:
  - sender
  - path
  - interface
  - signal/member
- Pretty-print signal arguments.
- Pause/resume signal stream.
- Clear signal log.
- Save signal log to file.
- Copy equivalent `busctl monitor` or match rule.

## Search and Filtering

- Fuzzy search bus names.
- Filter object paths, interfaces, methods, properties, and signals.
- Global search across introspected services.
- Jump to path/interface/member by text.
- Show recently visited items.
- Bookmark frequently used services/methods/properties.

## Introspection Improvements

- Cache introspection results per bus/service/path.
- Refresh/re-introspect current node.
- Detect and display introspection errors per object.
- Handle services with partial or missing introspection.
- Support manual object path entry when introspection is incomplete.
- Show raw introspection XML for debugging.
- Export introspection data to XML or JSON.

## Bus/Service Diagnostics

- Show name owner via `GetNameOwner`.
- Show activatable names via `ListActivatableNames`.
- Start activatable service via `StartServiceByName`.
- Show service process info when supported:
  - Unix PID
  - Unix user ID
  - security label
- Show whether a name currently has an owner.
- Watch `NameOwnerChanged` events.
- Highlight services appearing/disappearing live.

## Results and Logs

- Add a result pane for method/property responses.
- Pretty-print D-Bus values by type.
- Support compact and expanded rendering for nested values.
- Keep an interaction log:
  - method calls
  - property reads/writes
  - errors
  - signals
- Save logs to file.
- Copy selected result to clipboard when available.

## Command Interop

- Generate equivalent commands for selected interactions:
  - `busctl introspect`
  - `busctl call`
  - `busctl get-property`
  - `busctl set-property`
  - `busctl monitor`
  - `gdbus call`
- Allow opening the generated command in `$EDITOR` or copying it.
- Optional command palette for common actions.

## UX

- Keyboard shortcuts for common actions:
  - refresh
  - search
  - call
  - get property
  - subscribe signal
  - copy command
  - open raw XML
- Help overlay with all keybindings.
- Status bar showing current bus/service/path/interface.
- Loading states for slow/introspection-heavy services.
- Non-blocking calls so the UI remains responsive.
- Clear error messages and retry actions.
- Configurable theme/colors.

## Safety

- Clearly mark potentially mutating operations:
  - method calls with side effects
  - writable property changes
  - service activation
- Confirm before calling methods unless marked safe/no-arg/read-only.
- Confirm before setting properties.
- Show call target and arguments before execution.
- Optional read-only mode.

## Configuration

- Default bus selection.
- Favorite services/object paths.
- Custom aliases for common calls.
- Theme configuration.
- Keybinding configuration.
- Cache location/settings.

## Testing Ideas

- Unit tests for introspection XML parsing.
- Unit tests for D-Bus signature parsing.
- Unit tests for argument string parsing.
- Unit tests for pane navigation and scroll behavior.
- Golden tests for rendering selected panes/details.
- Integration tests using a small fake/test D-Bus service.
