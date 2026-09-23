# Disposable bookshelf-card layouts

Question: how should cover recognition, long titles and reading actions share space on a narrow bookshelf?

From the repository root:

```sh
python3 -m http.server 4181 --bind 127.0.0.1 --directory frontend
```

Open http://localhost:4181/prototypes/bookshelf-cards/?variant=A . Stop with Ctrl+C.

- A: cover gallery, two columns on phones, reading action below each book.
- B: compact cover-and-text cards with a progress/action footer.
- C: reading-focused rows, wide title space, smaller artwork.

Bottom arrows or keyboard left/right switch variants. Width and English/Traditional Chinese controls exercise mixed title lengths, unread state and different progress values. Requested width is capped to the browser window. Extra latest-chapter information appears on wider screens.

Read-only mock: all books are synthetic; navigation, filtering and book actions are inert. Shared artwork, tokens and button styles are referenced from the existing frontend. The surrounding shelf/Continue Reading area is simplified context, not a proposed replacement for those components. No app routes, backend, persistence or dependencies are changed.

Delete this directory to remove the experiment. Nothing imports it into the production build. Implement a selected design properly in the shelf component rather than promoting this mock verbatim.
