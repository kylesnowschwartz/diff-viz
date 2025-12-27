// Package render provides diff visualization renderers.
//
// Each renderer implements the Renderer interface and produces
// a different visualization format for git diff statistics.
//
// Available renderers:
//   - TreeRenderer: Indented tree with file stats
//   - CollapsedRenderer: Single-line summary per directory
//   - SmartSparklineRenderer: Depth-2 aggregated sparkline
//   - TopNRenderer: Top N files by change size
//   - IcicleRenderer: Horizontal icicle chart
//   - BracketsRenderer: Nested brackets visualization
//
// Use ValidModes and IsValidMode to enumerate and validate mode names.
package render
