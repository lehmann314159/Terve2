// selection.js — Capture word clicks and phrase selections, send to /analyze via htmx.

// Clear previous highlights on mousedown, deferred to avoid
// reflow that can shift the browser's selection anchor point.
document.addEventListener('mousedown', function(e) {
    var body = document.getElementById('article-body');
    if (!body || !body.contains(e.target)) return;
    requestAnimationFrame(function() {
        body.querySelectorAll('.word.selected').forEach(function(w) {
            w.classList.remove('selected');
        });
    });
});

document.addEventListener('mouseup', function(e) {
    var body = document.getElementById('article-body');
    if (!body || !body.contains(e.target)) return;

    var sel = window.getSelection();
    var text = sel.toString().trim();

    // Expand partial selections to full word boundaries
    if (text) {
        if (sel.rangeCount > 0) {
            var range = sel.getRangeAt(0);

            // Expand start to the left until whitespace
            var startNode = range.startContainer;
            var startOffset = range.startOffset;
            if (startNode.nodeType === Node.TEXT_NODE) {
                var txt = startNode.textContent;
                while (startOffset > 0 && !/\s/.test(txt[startOffset - 1])) {
                    startOffset--;
                }
                range.setStart(startNode, startOffset);
            }

            // Expand end to the right until whitespace
            var endNode = range.endContainer;
            var endOffset = range.endOffset;
            if (endNode.nodeType === Node.TEXT_NODE) {
                var etxt = endNode.textContent;
                while (endOffset < etxt.length && !/\s/.test(etxt[endOffset])) {
                    endOffset++;
                }
                range.setEnd(endNode, endOffset);
            }

            text = range.toString().trim();
        }
    }

    // If no selection, check if a word span was clicked
    if (!text && e.target.classList.contains('word')) {
        text = e.target.textContent.trim();
    }

    if (!text) return;

    // Get sentence context from the parent paragraph div
    var context = '';
    var parent = e.target.closest('.paragraph, p, .tokenized-text');
    if (parent) {
        context = parent.textContent.trim();
    }

    // Save the range before clearing browser selection, so we can
    // use it to determine which word spans to highlight.
    var savedRange = null;
    var hadDragSelection = sel.toString().trim() !== '';
    if (sel.rangeCount > 0) {
        savedRange = sel.getRangeAt(0).cloneRange();
    }

    // Clear browser text selection first to avoid double highlight
    sel.removeAllRanges();

    // Highlight selected word(s) via CSS class
    var words = body.querySelectorAll('.word');
    words.forEach(function(w) { w.classList.remove('selected'); });
    if (e.target.classList.contains('word') && !hadDragSelection) {
        e.target.classList.add('selected');
    } else if (savedRange) {
        words.forEach(function(w) {
            if (savedRange.intersectsNode(w)) {
                w.classList.add('selected');
            }
        });
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
});

// --- Paragraph bookmark ---

// Save a paragraph-level bookmark via POST.
function saveParaBookmark(bookID, chapterID, paragraph, btn) {
    var formData = new FormData();
    formData.append('chapter_id', chapterID);
    formData.append('paragraph', paragraph);
    fetch('/books/' + bookID + '/bookmark', {
        method: 'POST',
        body: formData
    }).then(function(resp) {
        if (resp.ok) {
            // Remove previous saved state
            document.querySelectorAll('.bookmark-btn.saved').forEach(function(b) {
                b.classList.remove('saved');
            });
            btn.classList.add('saved');
            // Update data attribute for scroll-on-return
            var cc = document.getElementById('chapter-content');
            if (cc) {
                cc.setAttribute('data-bookmark-paragraph', paragraph);
            }
        }
    });
}

// Scroll to bookmarked paragraph on load and after HTMX swaps.
(function() {
    function scrollToBookmark() {
        var cc = document.getElementById('chapter-content');
        if (!cc) return;
        var p = parseInt(cc.getAttribute('data-bookmark-paragraph'), 10);
        if (!p || p === 0) return;
        var target = document.getElementById('p-' + p);
        if (!target) return;
        target.classList.add('bookmarked');
        target.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }

    // On initial page load
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', scrollToBookmark);
    } else {
        scrollToBookmark();
    }

    // After HTMX swaps in new chapter content
    document.addEventListener('htmx:afterSwap', function(e) {
        if (e.detail.target && e.detail.target.id === 'chapter-content') {
            // Clear bookmark paragraph on chapter navigation (auto-save sets paragraph=0)
            var cc = document.getElementById('chapter-content');
            if (cc && !cc.getAttribute('data-bookmark-paragraph')) {
                return;
            }
        }
    });
})();
