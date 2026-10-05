# Document icon alignment and CRM favicon

- [x] Inspect both supplied screenshots and trace the icon sources.
- [x] Align logo, initials and page icon boxes independently of their child baselines.
- [x] Serve a conventional public ICO favicon using the existing CRM glyph; preserve the themed SVG.
- [x] Inspect the screenshots, decoded ICO asset and production DOM; run full checks and strict review.
- [x] Prepare the verified change for commit/push; record deployment verification in project memory.

Jaz chat previously used Google's favicon proxy, which returns 404 for CRM despite a working SVG favicon. The companion Jaz change tries the site's conventional favicon before that proxy. No new dependency or company data changes.

Full frontend check and Go build/vet/test pass, including real anonymous HTTP retrieval and decoding of SVG/ICO favicons. The side browser disconnected before the final alignment screenshot, so post-change visual verification remains unavailable. The correction uses middle alignment of equal icon boxes, which is independent of the child's baseline.

| Before | After |
| --- | --- |
| Record icon positions inherited different child baselines. | Fixed-size icon boxes share middle alignment. |
| CRM exposed only its themed SVG favicon. | A public ICO endpoint also supports conventional discovery. |
