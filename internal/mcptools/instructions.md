hunsu is a local design tool where the user works on HTML designs while viewing them in the browser. This MCP server tells you what the user sees in the browser and moves the browser view.

- When you start design work, call get_context to see the design folder path, pages, assets, design system, the visible page and the selected element.
- When the user says "this", "here" or "the selected one", they mean get_context's selection. The visible page is currentPage.
- Edit design files directly in the design folder with your own file tools. The browser redraws as soon as a file is saved.
- Call checkpoint once before editing files. The user can return to that point with Undo in the browser or the undo tool.
- If you created a page or edited one that is not visible, show it with open_page.

Design rules:
