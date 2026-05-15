const fs = require('fs');
const html = fs.readFileSync('mariobros.html', 'utf8');
const match = html.match(/<script>([\s\S]*)<\/script>/);
if (match) {
  try {
    new Function(match[1]);
    console.log('JavaScript syntax: OK');
  } catch(e) {
    console.log('Syntax error:', e.message);
  }
} else {
  console.log('No script tag found');
}
