# Disposable Continue Reading prototypes

Question: which narrow-screen composition best balances cover identity, reading context and actions?

From the repository root:

```sh
python3 -m http.server 4181 --bind 127.0.0.1 --directory frontend
```

Open http://localhost:4181/prototypes/continue-reading/ . Stop with Ctrl+C.

- **A:** compact side-by-side, all information beside the artwork.
- **B:** cover/title row, full-width reading context and actions below on mobile.
- **C:** cover/title/chapter row, separate progress/action footer.

Use the bottom arrows (or keyboard left/right) to switch. `?variant=B` links directly to a variant. Choose preview widths and typical/long/Traditional Chinese sample content above the preview. Width is capped to your browser window. All book data is synthetic; book and navigation controls are intentionally inert. No backend, login, storage or application wiring.

Existing tokens, button styles and artwork are referenced rather than copied. The surrounding shelf is a simplified context mock, not an exact reproduction of the app shell. These files are not imported by the app or its production build.

Delete `frontend/prototypes/continue-reading/` to remove the entire experiment. No other files need reverting. Do not promote this mock directly into production; implement the chosen layout in ShelfView.vue after review.
