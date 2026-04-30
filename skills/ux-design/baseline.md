# UX Baseline

Universal best-practice rules and decision-axis defaults that ship with
the framework. Every project running `ux-design init` inherits these as
the starting point; the project's `docs/UX_DESIGN.md` declares overrides
explicitly.

This file is **placeholder**. Authoring the substantive content is its
own design exercise — drafting the right defaults requires care, citation
discipline, and a coherent set of choices across the decision axes. The
`ux-design` skill loads this file expecting structured content; the
sections below name what should live here without yet supplying it.

---

## Standards declaration

The default standards baseline. Projects may override in their
`docs/UX_DESIGN.md` preamble.

- **Accessibility**: WCAG 2.1 AA.
- **Browsers**: latest two versions of Chrome, Firefox, Safari, Edge.
- **Touch-target floor**: 36×36 desktop / 44×44 if any touch use is
  anticipated.

## Decision axes (defaults pre-set to best practice)

For each axis: the question, the recommended default, and a one-line
citation of why it's the default. The skill renders these during init
mode with the default pre-selected; the human accepts (zero friction)
or overrides (records reason in the doc).

- **Validation timing**: on-blur, plus revalidate-on-change after the
  first failed submit. *(Cited: prevents premature error states while
  preserving real-time feedback once errors exist.)*
- **Modal dismissal — destructive confirms**: explicit only (Esc and
  outside-click do not close). *(Prevents accidental dismissal of
  irreversible actions.)*
- **Modal dismissal — non-destructive**: Esc and outside-click both
  close. *(Standard expected behaviour for browse / inspect dialogs.)*
- **Required indicator policy**: ratio-based — if ≥60% of fields are
  required, mark the optional ones; otherwise mark the required ones
  with `*`. *(Reduces visual noise on forms where most fields are
  required.)*
- **Row click semantics**: row click does nothing; actions go through
  an explicit kebab menu or a primary CTA. *(Prevents accidental
  navigation; makes the interaction surface explicit.)*
- **Toast position**: top-right. *(Out of the way of primary content;
  consistent with most modern web UIs.)*
- **Toast persistence**: importance-based — must-see toasts stay
  until dismissed; safe-to-miss toasts auto-close at 5s. Severity
  alone does not determine persistence. *(An info-severity toast
  that says "your export is ready" is must-see; an error-severity
  toast about an ambient sensor is safe-to-miss.)*
- **Empty-state richness**: context-dependent — first-run states get
  illustrations and onboarding copy; filtered-empty states get bare
  helper text. *(Empty by chance ≠ empty by intent.)*
- **Destructive action recovery**: confirm-modal at minimum;
  undo-toast as a project-level upgrade where data loss is genuinely
  recoverable. *(Confirmation prevents the action; undo recovers
  from the action — different problems.)*

---

## Foundations

Token names, scale rules, iconography conventions. (To be authored.)

## Layout & navigation

App-shell shape, primary nav patterns, breadcrumb policy, page-title
hierarchy. (To be authored.)

## Forms

Field grouping, label placement, input sizing, helper text vs error
text, multi-step forms, autosave vs explicit-save. (To be authored.)

## Buttons

Primary / secondary / tertiary semantics, size scale, disabled-state
handling, destructive-action styling, loading states. (To be authored.)

## Tables and lists

Row density, column sorting, row selection, bulk actions, pagination
vs infinite scroll vs virtualised. (To be authored.)

## Modals

Open/close transitions, focus trap, body scroll lock, stack policy
(can a modal open another modal?). (To be authored.)

## Cards and panels

Elevation scale, padding scale, card-as-link vs card-with-buttons,
collapsible-panel patterns. (To be authored.)

## Feedback

Toasts, banners, error pages, status indicators, progress bars,
skeleton-vs-spinner choices. (To be authored.)

## Data display

Numbers, dates, currency, units, truncation, tooltip-vs-expand for
overflow, empty-cell rendering. (To be authored.)

## Dark mode

Token-pair strategy, image handling, brand-colour adaptation. (To be
authored — applicable only if the project has dark mode in scope.)

## Accessibility

Beyond the WCAG AA standard above: focus-visible policy, reduced-motion
support, screen-reader-only text conventions, live-region patterns.
(To be authored.)

---

## How this file is consumed

The `ux-design` skill reads this file at the start of every session.
In **init mode** the agent presents each axis above with the default
pre-selected. In **extend** and **add-deviation** modes the agent uses
this file as the citation source when it explains *why* a rule or
default is what it is.

When the skill is updated to flesh out the placeholder sections, the
content should follow the format used in the project-wide
`docs/UX_DESIGN.md`: stable rule IDs (`R-<AREA>-<TOPIC>-<NN>`), RFC
2119 MUST/SHOULD/MAY semantics, anti-example pairs, file-path scopes
where lintable.
