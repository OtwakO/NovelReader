# Shared presentation

- `styles/tokens.css` owns typography roles: page, section, subheading, body, small and caption; regular and strong weights. Use roles rather than choosing new font sizes/weights per view. Reader prose and font previews retain their user-controlled sizing and line spacing.
- `FeatureScaffold` owns page headings, descriptions and the optional `actions` slot. Keep page actions there instead of introducing a second header style.
- `AppDisclosure` owns the native `<details>/<summary>` interaction, chevron, focus treatment and content padding. Put the label in `#summary`. Use `v-model:open` or `@toggle` only when the feature needs open state (for example, lazy data loading). The component does not fetch, retain or discard feature data.
- Ordinary input/select/textarea values use body size and regular weight, independently of smaller or emphasized labels. This also avoids undersized mobile form text.
- `AppButton` owns button semantics and busy/disabled state. `styles/controls.css` owns its appearance; navigational `RouterLink`s can use `app-button app-button--secondary` (or another existing variant) without pretending to be buttons.
- `app-actions` provides wrapping, center-aligned action rows. Feature CSS may position the row or change its responsive layout; avoid duplicating its baseline gap/alignment.

Feature components own their data, mutations, polling and error recovery. Shared components own presentation and basic interaction only. Prefer a shared CSS rule for simple layout, a component for repeated markup/interaction, and no wrapper whose only job is passing calls through.
