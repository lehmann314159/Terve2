// selection.js — Capture word clicks and phrase selections, send to /analyze via htmx.

// Clear previous highlights as soon as a new interaction begins.
document.addEventListener('mousedown', function(e) {
    var body = document.getElementById('article-body');
    if (!body || !body.contains(e.target)) return;
    body.querySelectorAll('.word.selected').forEach(function(w) {
        w.classList.remove('selected');
    });
});

document.addEventListener('mouseup', function(e) {
    var body = document.getElementById('article-body');
    if (!body || !body.contains(e.target)) return;

    var sel = window.getSelection();
    var text = sel.toString().trim();

    // If no selection, check if a word span was clicked
    if (!text && e.target.classList.contains('word')) {
        text = e.target.textContent.trim();
    }

    if (!text) return;

    // Get sentence context from the parent paragraph or tokenized-text div
    var context = '';
    var parent = e.target.closest('p, .tokenized-text');
    if (parent) {
        context = parent.textContent.trim();
    }

    // Highlight selected word(s)
    var words = body.querySelectorAll('.word');
    words.forEach(function(w) { w.classList.remove('selected'); });
    if (e.target.classList.contains('word') && !sel.toString().trim()) {
        e.target.classList.add('selected');
    } else {
        // Multi-word drag selection: highlight all word spans in range
        if (sel.rangeCount > 0) {
            var range = sel.getRangeAt(0);
            words.forEach(function(w) {
                if (range.intersectsNode(w)) {
                    w.classList.add('selected');
                }
            });
        }
    }

    // Send to analysis endpoint
    htmx.ajax('POST', '/analyze', {
        target: '#analysis-panel',
        values: { text: text, context: context }
    });

    // Show loading state
    var panel = document.getElementById('analysis-panel');
    if (panel) {
        panel.innerHTML = '<h3>Analysis</h3><div class="loading"><div class="spinner"></div><p>Analyzing...</p></div>';
    }

    // Clear browser text selection (visual highlight stays via .selected class)
    sel.removeAllRanges();
});
