const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.launch();
  const context = await browser.newContext({
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36'
  });
  const page = await context.newPage();
  page.on('console', msg => console.log('BROWSER CONSOLE:', msg.text()));
  page.on('pageerror', error => console.log('BROWSER ERROR:', error.message));
  await page.goto('http://localhost:5173/');
  await page.waitForTimeout(2000);
  await page.keyboard.press('Meta+KeyK');
  await page.waitForTimeout(500);
  console.log('Dialog visible:', await page.isVisible('[placeholder="Search docs"]'));
  await browser.close();
})();
