// Patch all contacts sub-pages to use unified extractList()
const fs = require('fs');
const path = require('path');
const pagesDir = 'e:/traework/jizhang/web/src/features/contacts/pages';

const files = fs.readdirSync(pagesDir).filter(f => f.endsWith('.vue'));

// The unified extractList function (uses const, placed after imports)
const helperFn = `
// 统一从 API 响应里提取数组（兼容 data / data.items / 直接数组）
function extractList(r) {
  if (r == null) return [];
  if (Array.isArray(r)) return r;
  if (Array.isArray(r.data)) return r.data;
  if (r.data && Array.isArray(r.data.items)) return r.data.items;
  return [];
}
`;

for (const f of files) {
  const fp = path.join(pagesDir, f);
  let c = fs.readFileSync(fp, 'utf8');
  const orig = c;

  // Remove old inline extract patterns like:
  //   parties.value = Array.isArray(p) ? p : (p as any).data || []
  //   parties.value = Array.isArray(p) ? p : ((p as any).data?.items ?? (p as any).data) || []
  c = c.replace(/parties\.value\s*=\s*Array\.isArray\([a-zA-Z]\)\s*\?\s*[a-zA-Z]\s*:\s*\([^)]*\)\s*\|\|\s*\[\]/g, 'parties.value = extractList(p)');
  c = c.replace(/receivables\.value\s*=\s*Array\.isArray\([a-zA-Z]\)\s*\?\s*[a-zA-Z]\s*:\s*\([^)]*\)\s*\|\|\s*\[\]/g, 'receivables.value = extractList(r)');
  c = c.replace(/contracts\.value\s*=\s*Array\.isArray\([a-zA-Z]\)\s*\?\s*[a-zA-Z]\s*:\s*\([^)]*\)\s*\|\|\s*\[\]/g, 'contracts.value = extractList(c)');

  // Add helperFn after last "import" line (if not already present)
  if (!c.includes('function extractList(r)')) {
    const lastImport = c.lastIndexOf('import ');
    const lineEnd = c.indexOf('\n', lastImport);
    c = c.slice(0, lineEnd + 1) + helperFn + c.slice(lineEnd + 1);
  }

  if (c !== orig) {
    fs.writeFileSync(fp, c);
    console.log('PATCHED:', f);
  } else {
    console.log('UNCHANGED:', f);
  }
}
