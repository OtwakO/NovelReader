async page => {
  const viewports = [
    { name: 'phone-320', width: 320, height: 700 },
    { name: 'phone-390', width: 390, height: 844 },
    { name: 'landscape-667', width: 667, height: 375 },
    { name: 'tablet-768', width: 768, height: 1024 },
    { name: 'laptop-1024', width: 1024, height: 768 },
    { name: 'desktop-1440', width: 1440, height: 900 }
  ];
  const results = [];

  async function inspect(route, viewport, setup) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await page.reload();
    if (setup) await setup();
    await page.waitForTimeout(80);
    const metrics = await page.evaluate(() => {
      const viewportWidth = document.documentElement.clientWidth;
      const all = [...document.querySelectorAll('body *')];
      const overflowers = all.filter(el => {
        const style = getComputedStyle(el);
        if (style.position === 'fixed' || style.position === 'absolute') return false;
        const rect = el.getBoundingClientRect();
        return rect.right > viewportWidth + 1 || rect.left < -1;
      }).slice(0, 8).map(el => ({ tag: el.tagName, className: String(el.className).slice(0, 80), text: (el.textContent || '').trim().slice(0, 50) }));
      const tinyTargets = [...document.querySelectorAll('button, select, a[href]')].filter(el => {
        const rect = el.getBoundingClientRect();
        const style = getComputedStyle(el);
        return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0 && (rect.width < 44 || rect.height < 44);
      }).slice(0, 12).map(el => ({ tag: el.tagName, text: (el.textContent || el.getAttribute('aria-label') || '').trim().slice(0, 50), width: Math.round(el.getBoundingClientRect().width), height: Math.round(el.getBoundingClientRect().height) }));
      const detailHero = document.querySelector('.detail-hero');
      const cover = detailHero?.querySelector('.cover')?.getBoundingClientRect();
      const copy = detailHero?.querySelector('.detail-hero-copy')?.getBoundingClientRect();
      const sheetElement = document.querySelector('.sheet');
      const sheet = sheetElement?.getBoundingClientRect();
      return {
        horizontalOverflow: document.documentElement.scrollWidth > viewportWidth + 1,
        overflowers,
        tinyTargets,
        detailTopDelta: cover && copy ? Math.round(Math.abs(cover.top - copy.top)) : null,
        sheetWithinViewport: sheet ? sheet.left >= -1 && sheet.right <= viewportWidth + 1 && sheet.top >= -1 && sheet.height <= innerHeight + 1 && getComputedStyle(sheetElement).overflowY === 'auto' : null,
        bodyText: document.body.innerText.slice(0, 80)
      };
    });
    results.push({ route, viewport: viewport.name, ...metrics });
  }

  for (const viewport of viewports) {
    await inspect('home', viewport);
    await inspect('discover', viewport, async () => page.getByRole('button', { name: /发现/ }).click());
    await inspect('search', viewport, async () => page.getByRole('button', { name: /搜索/ }).last().click());
    await inspect('detail', viewport, async () => page.getByRole('button', { name: '书籍详情' }).click());
    await inspect('reader', viewport, async () => page.getByRole('button', { name: '继续阅读' }).click());
    await inspect('source-sheet', viewport, async () => {
      await page.getByRole('button', { name: '继续阅读' }).click();
      await page.getByRole('button', { name: /书源/ }).click();
    });
  }
  return results;
}
