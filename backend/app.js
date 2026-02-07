var createError = require('http-errors');
var express = require('express');
var path = require('path');
var cookieParser = require('cookie-parser');
var logger = require('morgan');
var session = require('express-session');
var passport = require('passport');
var GitHubStrategy = require('passport-github2').Strategy;
var dotenv = require('dotenv');
var { createProxyMiddleware } = require('http-proxy-middleware');

var containersRouter = require('./routes/containers');

var app = express();

dotenv.config({ path: path.join(__dirname, '.env') });

var isProd = app.get('env') === 'production';
var authDisabled = String(process.env.NO_AUTH || '').toLowerCase() === 'true';
var frontendDevUrl = process.env.FRONTEND_DEV_URL || 'http://localhost:5173';
var frontendProxy = null;
var githubClientId = process.env.GITHUB_CLIENT_ID;
var githubClientSecret = process.env.GITHUB_CLIENT_SECRET;
var githubCallbackUrl = process.env.GITHUB_CALLBACK_URL;
if (!githubCallbackUrl && !isProd) {
  githubCallbackUrl = 'http://localhost:3000/auth/github/callback';
}

if (!authDisabled) {
  if (!githubClientId || !githubClientSecret) {
    throw new Error('Missing GITHUB_CLIENT_ID or GITHUB_CLIENT_SECRET');
  }
  if (isProd && !githubCallbackUrl) {
    throw new Error('Missing GITHUB_CALLBACK_URL in production');
  }
}

app.use(logger('dev'));
app.use(express.json());
app.use(express.urlencoded({ extended: false }));
app.use(cookieParser());
app.use(express.static(path.join(__dirname, 'public')));

if (isProd) {
  app.use(express.static(path.join(__dirname, '..', 'frontend', 'dist')));
}

if (!isProd) {
  frontendProxy = createProxyMiddleware({
    target: frontendDevUrl,
    changeOrigin: true,
    ws: true
  });
  app.use(function(req, res, next) {
    if (req.path.startsWith('/api') || req.path.startsWith('/auth')) {
      return next();
    }
    return frontendProxy(req, res, next);
  });
}

app.use(session({
  secret: process.env.SESSION_SECRET || 'dev-session-secret',
  resave: false,
  saveUninitialized: false,
  cookie: {
    httpOnly: true,
    sameSite: 'lax',
    secure: isProd
  }
}));

app.use(passport.initialize());
app.use(passport.session());

passport.serializeUser(function(user, done) {
  done(null, user);
});

passport.deserializeUser(function(obj, done) {
  done(null, obj);
});

if (!authDisabled) {
  passport.use(new GitHubStrategy({
    clientID: githubClientId,
    clientSecret: githubClientSecret,
    callbackURL: githubCallbackUrl
  }, function(accessToken, refreshToken, profile, done) {
    return done(null, {
      id: profile.id,
      username: profile.username,
      displayName: profile.displayName
    });
  }));
}

function ensureAuth(req, res, next) {
  if (authDisabled) {
    req.user = { id: 'dev', username: 'dev', displayName: 'Dev User' };
    return next();
  }
  if (req.isAuthenticated && req.isAuthenticated()) {
    return next();
  }
  return res.status(401).json({ ok: false, error: 'Authentication required' });
}

app.get('/auth/github', function(req, res, next) {
  if (authDisabled) {
    return res.redirect('/');
  }
  return passport.authenticate('github', { scope: ['read:user'] })(req, res, next);
});

app.get('/auth/github/callback', function(req, res, next) {
  if (authDisabled) {
    return res.redirect('/');
  }
  return passport.authenticate('github', { failureRedirect: '/auth/failed' })(req, res, next);
}, function(req, res) {
  res.redirect('/');
});

app.post('/auth/logout', function(req, res) {
  if (req.logout) {
    req.logout(function() {
      res.json({ ok: true });
    });
  } else {
    res.json({ ok: true });
  }
});

app.get('/auth/failed', function(req, res) {
  res.status(401).send('GitHub authentication failed.');
});

app.get('/api/me', function(req, res) {
  if (authDisabled) {
    return res.json({ ok: true, user: { id: 'dev', username: 'dev', displayName: 'Dev User' } });
  }
  if (req.isAuthenticated && req.isAuthenticated()) {
    return res.json({ ok: true, user: req.user });
  }
  return res.status(401).json({ ok: false, error: 'Not authenticated' });
});

app.use('/api/containers', ensureAuth, containersRouter);


function handleFrontendFallback(req, res) {
  if (!isProd) {
    return res.status(404).send('Not found.');
  }
  return res.sendFile(path.join(__dirname, '..', 'frontend', 'dist', 'index.html'));
}

// catch 404
app.use(function(req, res, next) {
  if (req.path.startsWith('/api') || req.path.startsWith('/auth')) {
    return next(createError(404));
  }
  return handleFrontendFallback(req, res);
});

// error handler
app.use(function(err, req, res, next) {
  if (req.path.startsWith('/api') || req.path.startsWith('/auth')) {
    res.status(err.status || 500);
    return res.json({
      ok: false,
      error: err.message || 'Server error'
    });
  }
  if (!isProd) {
    return res.status(500).send('Server error.');
  }
  return res.redirect('/error');
});

module.exports = app;
