/*
 * Client-side search for the documentation.
 *
 * The index is search.json, written by the build: every page with its title,
 * summary and section headings. It is a few kilobytes, so it is fetched once
 * lazily — on the first keystroke, not on page load — and matched in the
 * browser. There is no search server and no dependency.
 *
 * Matching is deliberately simple: every term must appear in the page title,
 * a heading or the summary, and the results are scored by where they matched.
 * A documentation search that cannot find "storage.root" or "SSO" is worse
 * than no search box, so titles and headings outrank the summary and an exact
 * heading match outranks a partial one.
 */

(function () {
  'use strict';

  var input = document.getElementById('search');
  var panel = document.getElementById('search-results');
  if (!input || !panel) {
    return;
  }

  var index = null;
  var loading = null;
  var active = -1;
  var results = [];

  // The index sits at the site root; the page may be in a subfolder.
  // This must not be called `root`: the theme block further down also wants a
  // "root element", and a second `var root` there silently reassigned this one
  // to <html>, so the index was fetched from
  // "/configuration/[object HTMLHtmlElement]search.json" and every search
  // came back empty.
  var siteRoot = document.currentScript
    ? document.currentScript.src.replace(/search\.js.*$/, '')
    : '';

  function load() {
    if (index) {
      return Promise.resolve(index);
    }
    if (!loading) {
      loading = fetch(siteRoot + 'search.json')
        .then(function (res) { return res.json(); })
        .then(function (data) {
          index = data;
          return index;
        })
        .catch(function () {
          index = [];
          return index;
        });
    }
    return loading;
  }

  function normalise(s) {
    return (s || '').toLowerCase();
  }

  // search matches one term against the index and returns scored hits.
  //
  // The body text is searched too, and a match names the section it is in, so
  // "logo" opens the page at the section that explains it rather than at the
  // top. Titles and headings still outrank prose: a page *called* SSO is a
  // better answer than a page that mentions it once.
  function search(terms) {
    var hits = [];
    index.forEach(function (page) {
      var title = normalise(page.title);
      var summary = normalise(page.summary);
      var sections = page.sections || [];
      var score = 0;
      var matchedAll = terms.every(function (term) {
        var inTitle = title.indexOf(term) !== -1;
        var inSummary = summary.indexOf(term) !== -1;
        var heading = (page.headings || []).find(function (h) {
          return normalise(h.text).indexOf(term) !== -1;
        });
        var inBody = sections.some(function (s) {
          return normalise(s.text).indexOf(term) !== -1;
        });
        if (inTitle) score += 10;
        if (heading) score += 6;
        if (inSummary) score += 3;
        // Prose is the weakest signal, but it is what makes the search find
        // "logo" at all.
        if (inBody) score += 1;
        return inTitle || inSummary || !!heading || inBody;
      });
      if (!matchedAll) {
        return;
      }
      var pageURL = (siteRoot + page.url).replace(/\/$/, '/index.html') || 'index.html';
      // The page itself first, then its matching sections, so "sso" offers
      // both the page and the section that explains it.
      hits.push({
        title: page.title,
        url: pageURL,
        summary: page.summary,
        score: score,
        heading: ''
      });
      (page.headings || []).forEach(function (h) {
        if (terms.every(function (term) { return normalise(h.text).indexOf(term) !== -1; })) {
          hits.push({
            title: page.title,
            url: pageURL + '#' + h.id,
            summary: h.text,
            score: score + 4,
            heading: h.text
          });
        }
      });
      // Sections whose prose matched, with a snippet around the term so the
      // reader can see why it was offered.
      //
      // A deeper heading is the better destination: "The logo" is an h3 inside
      // "frontend_config", and opening the h2 drops the reader at the top of a
      // long table. So a match inside a section ranks with its depth, and the
      // section a match belongs to wins over its parent's accumulated text.
      sections.forEach(function (s) {
        var text = normalise(s.text);
        var every = terms.every(function (term) { return text.indexOf(term) !== -1; });
        if (!every) {
          return;
        }
        hits.push({
          title: page.title,
          url: s.id ? pageURL + '#' + s.id : pageURL,
          summary: snippet(s.text, terms[0]),
          score: score + (s.id ? 2 : 0) + (s.level === 3 ? 1 : 0),
          heading: s.heading || ''
        });
      });
    });
    hits.sort(function (a, b) { return b.score - a.score; });
    return hits.slice(0, 12);
  }

  // snippet returns the words around the first match, so a result shows the
  // sentence the term was found in rather than the start of the section.
  function snippet(text, term) {
    var at = normalise(text).indexOf(term);
    if (at < 0) {
      return text.slice(0, 120);
    }
    var start = Math.max(0, at - 40);
    var end = Math.min(text.length, at + term.length + 80);
    var out = text.slice(start, end).trim();
    return (start > 0 ? '… ' : '') + out + (end < text.length ? ' …' : '');
  }

  function render() {
    if (results.length === 0) {
      panel.innerHTML = '<p class="empty">No page matches that.</p>';
      panel.classList.remove('hidden');
      return;
    }
    var html = '<ul role="listbox">';
    results.forEach(function (hit, i) {
      var cls = i === active ? ' class="active"' : '';
      var hint = hit.heading ? '<span class="hint">' + escapeHTML(hit.title) + '</span>' : '';
      var label = hit.heading || hit.summary;
      html += '<li><a' + cls + ' role="option" href="' + hit.url + '">' +
        '<span class="page">' + escapeHTML(label || hit.title) + '</span>' +
        (hit.heading ? hint : '') +
        '</a></li>';
    });
    html += '</ul>';
    panel.innerHTML = html;
    panel.classList.remove('hidden');
  }

  function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function close() {
    panel.classList.add('hidden');
    active = -1;
  }

  var timer = null;
  input.addEventListener('input', function () {
    clearTimeout(timer);
    var value = input.value.trim().toLowerCase();
    if (value === '') {
      close();
      return;
    }
    timer = setTimeout(function () {
      load().then(function () {
        results = search(value.split(/\s+/));
        active = -1;
        render();
      });
    }, 120);
  });

  input.addEventListener('keydown', function (e) {
    if (panel.classList.contains('hidden')) {
      return;
    }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      var step = e.key === 'ArrowDown' ? 1 : -1;
      active = (active + step + results.length) % results.length;
      render();
      var el = panel.querySelector('a.active');
      if (el) {
        el.scrollIntoView({ block: 'nearest' });
      }
    } else if (e.key === 'Enter' && results.length > 0) {
      var pick = results[active >= 0 ? active : 0];
      window.location.href = pick.url;
    } else if (e.key === 'Escape') {
      close();
    }
  });

  document.addEventListener('click', function (e) {
    if (!panel.contains(e.target) && e.target !== input) {
      close();
    }
  });

  // "/" focuses the search field, the shortcut a documentation site is
  // expected to have, unless the reader is already typing somewhere.
  document.addEventListener('keydown', function (e) {
    if (e.key === '/' && document.activeElement === document.body) {
      e.preventDefault();
      input.focus();
    }
  });

  var toggle = document.querySelector('.nav-toggle');
  if (toggle) {
    toggle.addEventListener('click', function () {
      var open = document.body.classList.toggle('nav-open');
      toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    });
  }

  // Theme. The stored choice is applied in the head, before the first paint,
  // so nothing flashes; this only handles the click and the logo swap.
  var rootElement = document.documentElement;
  var themeButton = document.querySelector('.theme-toggle');
  var logo = document.querySelector('.brand-logo');

  function currentTheme() {
    var set = rootElement.getAttribute('data-theme');
    if (set === 'light' || set === 'dark') {
      return set;
    }
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light';
  }

  function paintLogo() {
    if (!logo) {
      return;
    }
    var want = currentTheme() === 'dark' ? 'dark' : 'light';
    var src = logo.getAttribute('data-' + want);
    if (src && logo.getAttribute('src') !== src) {
      logo.setAttribute('src', src);
    }
  }

  if (themeButton) {
    themeButton.addEventListener('click', function () {
      var next = currentTheme() === 'dark' ? 'light' : 'dark';
      rootElement.setAttribute('data-theme', next);
      try {
        localStorage.setItem('filegator-docs-theme', next);
      } catch (e) {}
      paintLogo();
    });
  }

  // A reader who has not chosen follows the system, including when it changes
  // while the page is open.
  if (window.matchMedia) {
    var query = window.matchMedia('(prefers-color-scheme: dark)');
    var onChange = function () {
      if (!rootElement.getAttribute('data-theme')) {
        paintLogo();
      }
    };
    if (query.addEventListener) {
      query.addEventListener('change', onChange);
    } else if (query.addListener) {
      query.addListener(onChange);
    }
  }

  paintLogo();
})();
