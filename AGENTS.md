# Project Rules

## Code Standards

- Code must be clean, readable, and clearly structured at all times.
- Prefer self-documenting code: descriptive names, small focused functions, early returns over deep nesting.

## Comments

- `//` comments are forbidden. This applies to:
  - standalone comment lines
  - trailing (end-of-line) comments
  - comment blocks at the top of a file
  - doc comments on exported identifiers
  - comments inside JSON, JSONC, YAML, and config files
- When an explanation is genuinely required, use a `/* ... */` block comment placed on its own line above the code it describes.
- If a block comment would make the file noisier than the code is unclear, write no comment at all. Extract a well-named function instead.
- Never add comments that merely restate what the code does.

## Verification

- After every change, run the project's build, lint, and test commands. If none are configured, say so explicitly rather than reporting success.
- Do not claim a change works unless it was actually compiled or executed.
