// A small waxon in the browser: the demo library, with the keys that matter
// (j/k, gg/G, h/l, Enter, Space, n/p, +/-, f, S, N, v, l, ?). It copies how
// waxon looks and behaves; it doesn't run waxon, and it plays no audio.
(function () {
  "use strict";

  var screen = document.getElementById("demo-screen");
  var term = document.getElementById("demo");
  if (!screen || !term) return;

  // ---- The demo library (demo/data.go) ------------------------------------

  function t(name, artist, album, art, dur) {
    var p = dur.split(":");
    return { name: name, artist: artist, album: album, art: art, dur: +p[0] * 60 + +p[1] };
  }
  var A = {
    currents: "currents", nevermind: "nevermind", ram: "random-access-memories",
    purple: "purple-rain", dsotm: "dark-side-of-the-moon", rumours: "rumours",
    okc: "ok-computer", kob: "kind-of-blue",
  };
  var playlists = [
    { name: "Liked Songs", art: "liked-songs", tracks: [
      t("Let It Happen", "Tame Impala", "Currents", A.currents, "7:46"),
      t("Smells Like Teen Spirit", "Nirvana", "Nevermind", A.nevermind, "5:01"),
      t("Get Lucky", "Daft Punk", "Random Access Memories", A.ram, "6:09"),
      t("The Less I Know the Better", "Tame Impala", "Currents", A.currents, "3:36"),
      t("Purple Rain", "Prince", "Purple Rain", A.purple, "8:41"),
      t("Comfortably Numb", "Pink Floyd", "The Wall", A.dsotm, "6:22"),
      t("Dreams", "Fleetwood Mac", "Rumours", A.rumours, "4:14"),
      t("Paranoid Android", "Radiohead", "OK Computer", A.okc, "6:23"),
      t("So What", "Miles Davis", "Kind of Blue", A.kob, "9:22"),
      t("New Person, Same Old Mistakes", "Tame Impala", "Currents", A.currents, "6:03"),
      t("Wish You Were Here", "Pink Floyd", "Wish You Were Here", A.dsotm, "5:34"),
      t("The Chain", "Fleetwood Mac", "Rumours", A.rumours, "4:28"),
      t("Karma Police", "Radiohead", "OK Computer", A.okc, "4:21"),
      t("Blue in Green", "Miles Davis", "Kind of Blue", A.kob, "5:27"),
      t("Time", "Pink Floyd", "The Dark Side of the Moon", A.dsotm, "7:06"),
      t("Go Your Own Way", "Fleetwood Mac", "Rumours", A.rumours, "3:38"),
      t("Lucky", "Radiohead", "OK Computer", A.okc, "4:19"),
      t("Money", "Pink Floyd", "The Dark Side of the Moon", A.dsotm, "6:22"),
      t("No Surprises", "Radiohead", "OK Computer", A.okc, "3:48"),
    ] },
    { name: "Classic Rock", art: "classic-rock", tracks: [
      t("Purple Rain", "Prince", "Purple Rain", A.purple, "8:41"),
      t("When Doves Cry", "Prince", "Purple Rain", A.purple, "5:54"),
      t("Comfortably Numb", "Pink Floyd", "The Wall", A.dsotm, "6:22"),
      t("Dreams", "Fleetwood Mac", "Rumours", A.rumours, "4:14"),
      t("Time", "Pink Floyd", "The Dark Side of the Moon", A.dsotm, "7:06"),
      t("Smells Like Teen Spirit", "Nirvana", "Nevermind", A.nevermind, "5:01"),
      t("Come As You Are", "Nirvana", "Nevermind", A.nevermind, "3:39"),
      t("Go Your Own Way", "Fleetwood Mac", "Rumours", A.rumours, "3:38"),
    ] },
    { name: "Late Night Coding", art: "late-night-coding", tracks: [
      t("Paranoid Android", "Radiohead", "OK Computer", A.okc, "6:23"),
      t("Breathe (In the Air)", "Pink Floyd", "The Dark Side of the Moon", A.dsotm, "2:43"),
      t("Karma Police", "Radiohead", "OK Computer", A.okc, "4:21"),
      t("Wish You Were Here", "Pink Floyd", "Wish You Were Here", A.dsotm, "5:34"),
      t("Blue in Green", "Miles Davis", "Kind of Blue", A.kob, "5:27"),
      t("Get Lucky", "Daft Punk", "Random Access Memories", A.ram, "6:09"),
      t("Instant Crush", "Daft Punk", "Random Access Memories", A.ram, "5:37"),
      t("Let It Happen", "Tame Impala", "Currents", A.currents, "7:46"),
    ] },
    { name: "Jazz Essentials", art: "jazz-essentials", tracks: [
      t("So What", "Miles Davis", "Kind of Blue", A.kob, "9:22"),
      t("Blue in Green", "Miles Davis", "Kind of Blue", A.kob, "5:27"),
      t("Freddie Freeloader", "Miles Davis", "Kind of Blue", A.kob, "9:46"),
      t("All Blues", "Miles Davis", "Kind of Blue", A.kob, "11:33"),
      t("Flamenco Sketches", "Miles Davis", "Kind of Blue", A.kob, "9:26"),
    ] },
  ];
  var lyrics = [
    [0, "♪"], [2, "Lyrics, right here in your terminal"], [4, "synced line by line to the beat"],
    [6, "The line you're on lights up"], [8, "while the rest gently fade away"],
    [10, "Powered by lrclib — no account,"], [12, "no API key, nothing to set up"],
    [14, "Press l to flip back to the art"], [16, "Just press play and sing along"], [18, "♪"],
  ];

  // ---- Album art, drawn in blocks as a terminal shows it ------------------

  var art = {}; // name -> { url, color }
  function loadArt(name, done) {
    if (art[name]) { done && done(art[name]); return; }
    var img = new Image();
    img.onload = function () {
      var n = 20, c = document.createElement("canvas");
      c.width = c.height = n;
      var x = c.getContext("2d");
      x.drawImage(img, 0, 0, n, n);
      // The accent is the average of the cover's colourful pixels, lifted
      // until it reads on the dark Now Playing background.
      var d = x.getImageData(0, 0, n, n).data, sum = [0, 0, 0], count = 0;
      for (var i = 0; i < d.length; i += 4) {
        var max = Math.max(d[i], d[i + 1], d[i + 2]), min = Math.min(d[i], d[i + 1], d[i + 2]);
        if (max - min > 40 && max > 70) { sum[0] += d[i]; sum[1] += d[i + 1]; sum[2] += d[i + 2]; count++; }
      }
      var rgb = count ? sum.map(function (v) { return v / count; }) : [180, 180, 190];
      var top = Math.max.apply(null, rgb), lift = top < 200 ? 200 / top : 1;
      rgb = rgb.map(function (v) { return Math.round(Math.min(255, v * lift)); });
      art[name] = { url: c.toDataURL(), color: "rgb(" + rgb.join(",") + ")" };
      render();
      done && done(art[name]);
    };
    img.src = "media/art/" + name + ".jpg";
  }
  function artImg(name, cls) {
    var a = art[name];
    if (!a) { loadArt(name); return '<span class="art ' + cls + ' art-empty"></span>'; }
    return '<img class="art ' + cls + '" src="' + a.url + '" alt="">';
  }

  // ---- State --------------------------------------------------------------

  var s = {
    pane: "tracks", libCursor: 0, open: 0, cursor: 0, top: 0,
    playing: { pl: 0, i: 0, track: playlists[0].tracks[0] }, paused: false, pos: 6,
    vol: 65, muted: false, shuffle: false, liked: {}, mode: "waxon",
    view: "main", np: "art", help: false, toast: null, g: false,
    touched: false, hint: false,
  };
  playlists[0].tracks.forEach(function (tr) { s.liked[key(tr)] = true; });
  function key(tr) { return tr.name + "|" + tr.artist; }
  // What's playing is held by track, not by row: liking and unliking change
  // Liked Songs' rows under it. i is its row, for n and p.
  function current() { return s.playing && s.playing.track; }

  var toastTimer;
  function toast(html) {
    s.toast = html;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { s.toast = null; render(); }, 1600);
  }

  function play(pl, i) {
    s.playing = { pl: pl, i: i, track: playlists[pl].tracks[i] }; s.pos = 0; s.paused = false;
    loadArt(current().art, syncRecord);
  }
  function skip(d) {
    if (!s.playing) return;
    var list = playlists[s.playing.pl].tracks, i = s.playing.i;
    if (d < 0 && s.pos > 3) { s.pos = 0; return; }
    if (!list.length) return;
    i = s.shuffle && d > 0 ? Math.floor(Math.random() * list.length) : (i + d + list.length) % list.length;
    play(s.playing.pl, i);
  }

  // ---- Keys ---------------------------------------------------------------

  // The track list shows as many rows as its pane has room for (measured
  // after drawing, in fitRows), as a terminal's height decides.
  var visibleRows = 12;
  var touchOnly = window.matchMedia("(hover: none)").matches;
  window.addEventListener("resize", render);
  function moveCursor(to) {
    if (s.pane === "lib") { s.libCursor = clamp(to, 0, playlists.length - 1); return; }
    var n = playlists[s.open].tracks.length;
    s.cursor = clamp(to, 0, n - 1);
    if (s.cursor < s.top) s.top = s.cursor;
    if (s.cursor >= s.top + visibleRows) s.top = s.cursor - visibleRows + 1;
  }
  function clamp(v, a, b) { return Math.max(a, Math.min(b, v)); }
  function cur() { return s.pane === "lib" ? s.libCursor : s.cursor; }

  function handle(k) {
    if (s.help) { if (k === "?" || k === "Escape" || k === "q") s.help = false; render(); return true; }
    var was = s.g; s.g = false;
    if (k === "g") { if (was) moveCursor(0); else s.g = true; render(); return true; }
    switch (k) {
      case " ": s.paused = !s.paused; syncRecord(); break;
      case "n": skip(1); break;
      case "p": skip(-1); break;
      case "+": case "=": s.vol = Math.min(100, s.vol + 5); s.muted = false; toast(meter()); break;
      case "-": s.vol = Math.max(0, s.vol - 5); toast(meter()); break;
      case "m": s.muted = !s.muted; toast(meter()); break;
      case "S": s.shuffle = !s.shuffle; toast("Shuffle " + (s.shuffle ? "on" : "off")); break;
      case "?": s.help = true; break;
      case "M":
        s.mode = s.mode === "waxon" ? "app" : "waxon";
        toast((s.mode === "app" ? "Controlling the Spotify app" : "Playing in waxon") + (current() ? ", from " + mmss(s.pos) : ""));
        break;
      case "N": s.view = s.view === "np" ? "main" : "np"; break;
      case "f": like(); break;
      default:
        if (s.view === "np") {
          if (k === "v" || k === "V") s.np = s.np === "vinyl" ? "art" : "vinyl";
          else if (k === "l") s.np = s.np === "lyrics" ? "art" : "lyrics";
          else if (k === "Escape" || k === "b" || k === "Backspace") s.view = "main";
          else return false;
          break;
        }
        if (k === "j" || k === "ArrowDown") moveCursor(cur() + 1);
        else if (k === "k" || k === "ArrowUp") moveCursor(cur() - 1);
        else if (k === "G") moveCursor(9999);
        else if (k === "h" || k === "ArrowLeft") s.pane = "lib";
        else if (k === "l" || k === "ArrowRight") s.pane = "tracks";
        else if (k === "Enter") {
          if (s.pane === "lib") { s.open = s.libCursor; s.cursor = 0; s.top = 0; s.pane = "tracks"; }
          else if (playlists[s.open].tracks.length) play(s.open, s.cursor);
        } else if (k === "Backspace" || k === "b") s.pane = "lib";
        else return false;
    }
    render();
    return true;
  }
  // f likes the selection (the playing track in Now Playing): it goes to the
  // top of Liked Songs, or out of it, as in waxon.
  function like() {
    var tr = s.view === "np" || s.pane === "lib" ? current() : playlists[s.open].tracks[s.cursor];
    if (!tr) return;
    var k = key(tr), liked = playlists[0].tracks, at = indexIn(liked, k);
    s.liked[k] = !s.liked[k];
    if (s.liked[k]) {
      if (at < 0) {
        liked.unshift(tr);
        if (s.open === 0) s.cursor++; // stay on the same track
      }
    } else if (at >= 0) {
      liked.splice(at, 1);
      if (s.open === 0 && s.cursor > at) s.cursor--;
    }
    if (s.open === 0) moveCursor(s.cursor);
    if (s.playing && s.playing.pl === 0) {
      var now = indexIn(liked, key(s.playing.track));
      // Unliked while playing from Liked Songs: it plays on, and n goes to
      // the track that followed it.
      s.playing.i = now >= 0 ? now : at - 1;
    }
    toast(s.liked[k] ? "♥ Added to Liked Songs" : "Removed from Liked Songs");
  }
  function indexIn(list, k) {
    for (var i = 0; i < list.length; i++) if (key(list[i]) === k) return i;
    return -1;
  }
  function meter() {
    var v = s.muted ? 0 : s.vol, n = Math.round(v / 10);
    return '<span class="meter">' + "▮".repeat(n) + '<i>' + "▮".repeat(10 - n) + "</i></span> " +
      (s.muted ? "muted" : v + "%");
  }

  function touch() {
    if (s.touched) return;
    s.touched = true; s.hint = false;
    term.classList.remove("invite");
    render();
  }
  term.addEventListener("keydown", function (e) {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    touch();
    if (handle(e.key)) { e.preventDefault(); flashKey(e.key); }
  });
  term.addEventListener("pointerdown", touch);

  // While the demo is on screen, its keys work from anywhere on the page
  // (not Space, which scrolls), so you can just start typing.
  var pageKeys = "jkgGhlnpfNvlMS?+-m";
  var onScreen = false;
  document.addEventListener("keydown", function (e) {
    if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
    if (!onScreen || document.activeElement === term || term.contains(e.target)) return;
    var el = document.activeElement;
    if (el && (el.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName))) return;
    if (e.key !== "Escape" && (e.key.length !== 1 || pageKeys.indexOf(e.key) < 0)) return;
    e.preventDefault();
    term.focus({ preventScroll: true });
    touch();
    handle(e.key);
    flashKey(e.key);
  });
  document.querySelectorAll("[data-key]").forEach(function (b) {
    b.addEventListener("click", function () {
      touch();
      var k = b.dataset.key;
      if (k === "gg") { handle("g"); handle("g"); } else handle(k);
      flashKey(k);
    });
  });
  function flashKey(k) {
    var sel = '[data-key="' + (k === "ArrowDown" ? "j" : k === "ArrowUp" ? "k" : k) + '"]';
    document.querySelectorAll(sel).forEach(function (b) {
      b.classList.remove("hit"); void b.offsetWidth; b.classList.add("hit");
    });
  }
  term.addEventListener("focus", function () { term.classList.add("live"); });
  term.addEventListener("blur", function () { term.classList.remove("live"); });

  // ---- Drawing ------------------------------------------------------------

  function esc(x) { return String(x).replace(/[&<>"]/g, function (c) { return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]; }); }
  function mmss(x) { x = Math.floor(x); return Math.floor(x / 60) + ":" + String(x % 60).padStart(2, "0"); }
  function total(pl) {
    var m = Math.round(pl.tracks.reduce(function (a, tr) { return a + tr.dur; }, 0) / 60);
    return m >= 60 ? Math.floor(m / 60) + "h " + (m % 60) + "m" : m + "m";
  }
  function bar(frac, width, color) {
    var n = Math.round(frac * width);
    return '<span class="bar" style="--c:' + color + '"><b style="width:' + (frac * 100) + '%"></b><em style="left:' + (frac * 100) + '%"></em></span>';
  }

  function libraryPane() {
    var rows = playlists.map(function (pl, i) {
      var cls = "lib-row" + (s.libCursor === i ? " sel" : "") + (s.open === i ? " open" : "");
      return '<div class="' + cls + '">' + artImg(pl.art, "tiny") +
        '<div><div>' + esc(pl.name) + '</div><div class="dim">' + pl.tracks.length + ' tracks</div></div></div>';
    }).join("");
    return '<section class="pane lib' + (s.pane === "lib" ? " focus" : "") + '"><div class="pane-title dim">LIBRARY</div>' + rows + "</section>";
  }

  function tracksPane() {
    var pl = playlists[s.open], now = current();
    var rows = pl.tracks.slice(s.top, s.top + visibleRows).map(function (tr, j) {
      var i = s.top + j;
      var isNow = s.playing && s.playing.pl === s.open && key(tr) === key(s.playing.track);
      var cls = "trow" + (i === s.cursor && s.pane === "tracks" ? " sel" : "") + (isNow ? " now" : "");
      return '<div class="' + cls + '"><span class="c-mark">' + (isNow ? (s.paused ? "‖" : "▶") : "") +
        '</span><span class="c-n">' + (i + 1) + '</span><span class="c-title">' + esc(tr.name) +
        (s.liked[key(tr)] && s.open !== 0 ? ' <span class="heart">♥</span>' : "") +
        '</span><span class="c-artist">' + esc(tr.artist) + '</span><span class="c-dur">' + mmss(tr.dur) + "</span></div>";
    }).join("");
    return '<section class="pane tracks' + (s.pane === "tracks" ? " focus" : "") + '">' +
      '<header class="tl-head">' + artImg(pl.art, "head") +
      '<div><div class="accent strong">' + esc(pl.name) + '</div><div class="dim">' + pl.tracks.length + " tracks · " + total(pl) + "</div></div></header>" +
      '<div class="trow th dim"><span class="c-mark"></span><span class="c-n">#</span><span class="c-title">Title</span><span class="c-artist">Artist</span><span class="c-dur">' + (term.offsetWidth < 640 ? "Time" : "Duration") + '</span></div>' +
      rows + "</section>";
  }

  function nowBar() {
    var tr = current();
    if (!tr) return '<footer class="nowbar"><span class="art big art-empty"></span><div class="dim">Nothing playing</div></footer>';
    return '<footer class="nowbar">' + artImg(tr.art, "big") + '<div class="nb-text">' +
      '<div class="accent strong">' + (s.paused ? "‖" : "▶") + " " + esc(tr.name) + "</div>" +
      '<div class="sec">' + esc(tr.artist) + " — " + esc(tr.album) + "</div>" +
      '<div class="nb-bar">' + bar(s.pos / tr.dur, 50, "var(--d-accent)") + "</div>" +
      "<div>" + mmss(s.pos) + " / " + mmss(tr.dur) + (s.shuffle ? ' <span class="dim">· shuffle</span>' : "") + "</div>" +
      '<div class="dim">♪ ' + (s.muted ? "muted" : s.vol + "%") + "  " + (s.mode === "app" ? "Spotify app" : "waxon") + "</div>" +
      '<div><span class="mode">NORMAL</span> <span class="dim">? help</span></div>' +
      "</div></footer>";
  }

  function nowPlaying() {
    var tr = current();
    if (!tr) return '<div class="np"><p class="dim">Nothing playing. Press N to go back.</p></div>';
    var a = art[tr.art], color = a ? a.color : "var(--d-accent)", body;
    if (s.np === "vinyl") {
      body = '<div class="np-vinyl"><div class="np-record' + (s.paused ? " stopped" : "") + '" style="--spin:' + spinDelay() + '"><div class="np-label">' +
        (a ? '<img src="' + a.url + '" alt="">' : "") + '</div></div><div class="np-arm" style="--p:' + (s.pos / tr.dur) + '"></div></div>';
    } else if (s.np === "lyrics") {
      var at = s.pos % 20, curLine = 0;
      lyrics.forEach(function (l, i) { if (l[0] <= at) curLine = i; });
      body = '<div class="np-lyrics">' + lyrics.map(function (l, i) {
        var d = Math.abs(i - curLine);
        return '<div class="ly' + (i === curLine ? " on" : "") + '" style="opacity:' + (i === curLine ? 1 : Math.max(0.25, 0.75 - d * 0.12)) + '">' + esc(l[1]) + "</div>";
      }).join("") + "</div>";
    } else {
      body = '<div class="np-art">' + artImg(tr.art, "huge") + "</div>";
    }
    return '<div class="np" style="--np:' + color + '">' + body +
      '<div class="np-title" style="color:' + color + '">' + esc(tr.name) + (s.liked[key(tr)] ? " ♥" : "") + "</div>" +
      '<div class="np-sub">' + esc(tr.artist) + " — " + esc(tr.album) + "</div>" +
      '<div class="np-bar">' + bar(s.pos / tr.dur, 40, color) + "</div>" +
      '<div class="np-time">' + mmss(s.pos) + " / " + mmss(tr.dur) + "</div>" +
      '<div class="np-hints dim">space play · n/p skip · f like · l lyrics · v vinyl · N close</div></div>';
  }

  function help() {
    var rows = [
      ["j / k", "move down / up"], ["gg / G", "top / bottom"], ["h / l", "library / tracks"],
      ["Enter", "open / play"], ["Space", "play / pause"], ["n / p", "next / previous"],
      ["+ / -", "volume"], ["f", "like"], ["S", "shuffle"], ["N", "Now Playing"],
      ["v / l", "vinyl / lyrics, in Now Playing"], ["Esc", "back from Now Playing or help"],
      ["M", "play in waxon ⇄ control the Spotify app"], ["?", "close help"],
    ];
    return '<div class="help"><div class="help-box"><div class="accent strong">Keys in this demo</div>' +
      rows.map(function (r) { return '<div class="help-row"><span class="accent">' + r[0] + "</span><span>" + r[1] + "</span></div>"; }).join("") +
      '<div class="dim help-more">The real waxon has many more: search, queue, playlists, devices, : commands.</div></div></div>';
  }

  // The screen is redrawn several times a second, which rebuilds the record;
  // each new one starts at the angle the last one had reached, from a clock
  // that runs only while music plays.
  var spinBase = 0, spinFrom = null;
  function spinDelay() {
    var playing = s.playing && !s.paused, now = performance.now();
    if (playing && spinFrom === null) spinFrom = now;
    if (!playing && spinFrom !== null) { spinBase += now - spinFrom; spinFrom = null; }
    var ms = spinBase + (spinFrom === null ? 0 : now - spinFrom);
    return "-" + Math.round(ms % 1800) + "ms";
  }

  function fitRows() {
    var pane = screen.querySelector(".pane.tracks"), th = pane && pane.querySelector(".trow.th");
    var row = pane && pane.querySelector(".trow:not(.th)");
    if (!th || !row || !pane.offsetParent) return;
    var bottom = pane.getBoundingClientRect().bottom - 8; // the pane's padding and border
    var rows = Math.max(3, Math.floor((bottom - th.getBoundingClientRect().bottom - 4) / row.offsetHeight));
    if (rows === visibleRows) return;
    visibleRows = rows;
    var n = playlists[s.open].tracks.length;
    s.top = clamp(Math.min(s.top, s.cursor), Math.max(0, s.cursor - rows + 1), Math.max(0, n - rows));
    render();
  }

  function render() {
    var main = s.view === "np" ? nowPlaying()
      : '<div class="panes">' + libraryPane() + tracksPane() + "</div>" + nowBar();
    screen.innerHTML = main + (s.help ? help() : "") + (s.toast ? '<div class="toast">' + s.toast + "</div>" : "") +
      (s.hint ? '<div class="invite-hint"><b>Your turn.</b> ' + (touchOnly
        ? "Tap the keys below."
        : "Press <kbd>j</kbd> <kbd>k</kbd> <kbd>Enter</kbd>, or <kbd>?</kbd> for every key.") + "</div>" : "");
    screen.dataset.view = s.view;
    syncModes();
    fitRows();
    screen.dataset.pane = s.pane;
  }

  // ---- The "Two ways to play" section shows where the demo is playing -----

  var modes = document.querySelector(".modes");
  function syncModes() {
    if (!modes) return;
    var tr = current();
    modes.dataset.mode = s.mode;
    modes.querySelectorAll("[data-np-title]").forEach(function (el) { el.textContent = tr ? tr.name : ""; });
    modes.querySelectorAll("[data-np-time]").forEach(function (el) { el.textContent = mmss(s.pos); });
  }

  // ---- The big record beside the headline follows what's playing ---------

  var record = document.querySelector(".record");
  function syncRecord() {
    if (!record) return;
    var tr = current(), a = tr && art[tr.art];
    record.classList.toggle("stopped", !tr || s.paused);
    var label = record.querySelector(".label");
    if (a && label) label.style.setProperty("--cover", "url(" + a.url + ")");
  }

  // ---- Themes (app/theme.go), as :theme switches them ---------------------

  var themes = {
    "spotify": ["#1db954", "#1f2430", "#e8e8e8", "#a9adb7", "#5d6472", "#363c4a"],
    "catppuccin-mocha": ["#a6e3a1", "#1e1e2e", "#cdd6f4", "#a6adc8", "#6c7086", "#45475a"],
    "catppuccin-latte": ["#40a02b", "#eff1f5", "#4c4f69", "#6c6f85", "#9ca0b0", "#bcc0cc"],
    "gruvbox": ["#b8bb26", "#282828", "#ebdbb2", "#bdae93", "#7c6f64", "#504945"],
    "tokyonight": ["#9ece6a", "#1a1b26", "#c0caf5", "#a9b1d6", "#565f89", "#3b4261"],
    "nord": ["#a3be8c", "#2e3440", "#eceff4", "#d8dee9", "#616e88", "#434c5e"],
    "dracula": ["#50fa7b", "#282a36", "#f8f8f2", "#bfbfbf", "#6272a4", "#44475a"],
    "rose-pine": ["#9ccfd8", "#191724", "#e0def4", "#908caa", "#6e6a86", "#403d52"],
  };
  var vars = ["--d-accent", "--d-bg", "--d-text", "--d-sec", "--d-dim", "--d-border"];
  document.querySelectorAll("[data-theme]").forEach(function (b) {
    b.addEventListener("click", function () {
      var p = themes[b.dataset.theme];
      vars.forEach(function (v, i) { term.style.setProperty(v, p[i]); });
      term.classList.toggle("light", b.dataset.theme === "catppuccin-latte");
      document.querySelectorAll("[data-theme]").forEach(function (o) { o.setAttribute("aria-pressed", o === b); });
      toast("Theme: " + b.dataset.theme);
      render();
    });
  });

  // ---- Showing that it's playable -----------------------------------------
  // The first time the demo is on screen it presses a few keys itself, with
  // the keycaps below going down, then hands over. Any input stops it.

  var script = [["j", 700], ["j", 450], ["Enter", 900], ["N", 1300], ["v", 2600], ["N", 900]];
  var reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var shown = false;
  function attract() {
    if (shown || s.touched) return;
    shown = true;
    if (reduced) { invite(); return; }
    var i = 0;
    (function step() {
      if (s.touched) return;
      if (i >= script.length) { invite(); return; }
      var k = script[i][0], wait = script[i][1];
      i++;
      setTimeout(function () {
        if (s.touched) return;
        handle(k); flashKey(k);
        step();
      }, wait);
    })();
  }
  function invite() {
    if (s.touched) return;
    s.hint = true;
    term.classList.add("invite");
    render();
  }
  if ("IntersectionObserver" in window) {
    new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        onScreen = e.intersectionRatio >= 0.4;
        if (e.intersectionRatio >= 0.6) setTimeout(attract, 600);
      });
    }, { threshold: [0, 0.4, 0.6] }).observe(term);
  }

  // ---- Time ---------------------------------------------------------------

  setInterval(function () {
    if (!s.playing || s.paused) return;
    s.pos += 0.25;
    if (s.pos >= current().dur) skip(1);
    if (!document.hidden) render();
  }, 250);

  playlists.forEach(function (pl) { loadArt(pl.art); });
  loadArt(current().art, syncRecord);
  render();
})();
