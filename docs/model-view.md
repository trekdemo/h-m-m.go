# `model.View` flow

`View` (main.go:110) renders `model` to a `tea.View` that Bubble Tea draws to
the terminal. It's a thin wrapper: guard against an unsized viewport,
delegate to `buildCanvas` for the actual compositing, then hand the
rendered string to the view's `Content` field.

```mermaid
flowchart TD
    Start["View() tea.View"] --> Init["v := tea.NewView(\"\")\nv.AltScreen = true\nv.MouseMode = MouseModeCellMotion"]
    Init --> Sized{"viewportW == 0 or\nviewportH == 0?"}
    Sized -->|yes| Empty["return v (empty content)"]
    Sized -->|no| Build["v.Content = buildCanvas().Render()"]

    subgraph BC["buildCanvas() *lipgloss.Canvas"]
        direction TB
        Init2["canvas := lipgloss.NewCanvas(viewportW, viewportH)"]
        Init2 --> SetEdge["setEdge(x, y, r, c) closure:\nconverts canvas coords -> viewport coords\n(x - offsetX, y - offsetY),\nno-op if outside viewport bounds,\nelse canvas.SetCell(...) with a uv.Cell"]

        SetEdge --> EdgesStraight["for each m.edges:\nedgeCells(e) -> straights, corners\ndraw all straights via setEdge()"]
        EdgesStraight --> EdgesCorner["draw all collected corners via setEdge()\n(after every straight, so a sibling's\ntrunk stroke can't stomp a shorter\nsibling's elbow)"]

        EdgesCorner --> Boxes["for each m.boxes:\nskip if fully off-screen"]
        Boxes --> Which{"i == m.selected?"}
        Which -->|yes| SelLines["use b.selLines\n(double border)"]
        Which -->|no| PlainLines["use b.lines\n(rounded border)"]
        SelLines --> Layer["wrap joined lines in\nlipgloss.NewStyle().Foreground(b.color),\nwrap that in a lipgloss.Layer\npositioned at (sx, sy)"]
        PlainLines --> Layer
        Layer --> Compose["canvas.Compose(lipgloss.NewCompositor(layers...))"]
    end

    Build --> Return(("return v"))
    Empty --> Return
```

## Notes

- **Two coordinate systems.** `m.boxes`/`m.edges` live in canvas
  coordinates (unbounded, can extend arbitrarily). `setEdge()` is the only
  place that translates edge coordinates into viewport-local coordinates by
  subtracting `offsetX`/`offsetY`, and clips anything outside `[0,
  viewportW) x [0, viewportH)`. Box layers are positioned the same way,
  via `X(sx).Y(sy)` where `sx, sy := b.x-m.offsetX, b.y-m.offsetY`.
- **Draw order matters.** Edges are written cell-by-cell onto the canvas
  before any box `Layer` is composed, so box borders always render on top
  of connector lines. Within edges, *every* edge's straight segments are
  drawn before *any* edge's corners — sibling edges sharing a trunk column
  can have one's vertical stroke pass through another's corner cell, so
  corners must be layered last.
- **Selection swaps the whole box, not just the border color** — the
  selected node uses `b.selLines` (pre-rendered with a double border),
  while every other box uses `b.lines` (rounded border). Both are computed
  once in `buildTree` (tree.go), not per-frame. Color is applied once per
  frame via `lipgloss.NewStyle().Foreground(b.color)` when building that
  box's `Layer` content.
- **Off-screen culling** happens at two granularities: the box loop in
  `buildCanvas` skips a box entirely up front if its whole bounding box
  doesn't overlap the viewport (cheap, avoids building a `Layer` for it at
  all); `lipgloss.Compositor.Draw` also skips drawing any composed layer
  whose bounds don't overlap the canvas, as a second safety net.
- **Edges are drawn as individual `uv.Cell`s** via `canvas.SetCell`, not as
  `Layer`s — connector strokes are scattered single cells, not rectangular
  blocks, so they don't fit the `Layer` model the way boxes do. Boxes, by
  contrast, are pre-rendered multi-line ANSI-free strings (from
  `buildTree`) that only need a uniform foreground color and a position, so
  they're a natural fit for `Layer` + `Compositor`.
- **`Canvas.Render()` trims trailing whitespace** (`uv.TrimSpace`),
  unlike the old manual `strings.Join`-based rendering, which preserved
  trailing spaces verbatim.
