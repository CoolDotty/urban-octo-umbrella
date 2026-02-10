var express = require('express');
var router = express.Router();
var { execFile } = require('child_process');
var crypto = require('crypto');
var { URL } = require('url');
var fs = require('fs');
var os = require('os');
var path = require('path');

var fsPromises = fs.promises;

function runPodman(args) {
  return new Promise(function(resolve, reject) {
    execFile('podman', args, { maxBuffer: 1024 * 1024 }, function(err, stdout, stderr) {
      if (err) {
        err.stderr = stderr;
        return reject(err);
      }
      resolve(stdout);
    });
  });
}

function runGit(args, options) {
  return new Promise(function(resolve, reject) {
    execFile('git', args, { maxBuffer: 1024 * 1024, cwd: options && options.cwd }, function(err, stdout, stderr) {
      if (err) {
        err.stderr = stderr;
        return reject(err);
      }
      resolve(stdout);
    });
  });
}

var APP_LABEL = 'com.urban-octo-umbrella.managed=true';
var REPO_URL_LABEL = 'com.urban-octo-umbrella.repo_url';
var REPO_PATH_LABEL = 'com.urban-octo-umbrella.repo_path';
var TUNNEL_NAME_LABEL = 'com.urban-octo-umbrella.tunnel_name';
var DEFAULT_IMAGE = 'mcr.microsoft.com/devcontainers/universal:latest';
var MAX_ACTIVE_CONTAINERS = 10;
var TUNNEL_WEB_BASE = process.env.CODE_TUNNEL_WEB_BASE || 'https://vscode.dev/tunnel';
var TUNNEL_ACCESS_TOKEN = process.env.CODE_TUNNEL_ACCESS_TOKEN || null;
var TUNNEL_REFRESH_TOKEN = process.env.CODE_TUNNEL_REFRESH_TOKEN || null;
var TUNNEL_NAME_PREFIX = process.env.CODE_TUNNEL_NAME_PREFIX || 'uou-';
var TUNNEL_DISABLE_TELEMETRY = String(process.env.CODE_TUNNEL_DISABLE_TELEMETRY || '').toLowerCase() === 'true';
var HOST_TUNNEL_DATA_DIR = process.env.CODE_TUNNEL_DATA_DIR_HOST || path.join(os.homedir(), '.vscode-cli');
var CONTAINER_TUNNEL_DATA_DIR = process.env.CODE_TUNNEL_DATA_DIR_CONTAINER || '/root/.vscode-cli';
var KEEP_FAILED_CONTAINERS = String(process.env.KEEP_FAILED_CONTAINERS || '').toLowerCase() === 'true';

var ANIMALS = [
  'lizard',
  'otter',
  'badger',
  'falcon',
  'tiger',
  'panda',
  'lemur',
  'gecko',
  'wolf',
  'eagle',
  'koala',
  'bison',
  'manta',
  'lynx',
  'sloth',
  'wren',
  'orca',
  'yak',
  'cobra',
  'ferret',
  'quokka',
  'heron',
  'raven',
  'moose',
  'viper',
  'fox',
  'marmot',
  'ibis',
  'puma',
  'coyote'
];

function getContainerLabel(container, key) {
  if (!container || !container.Labels) {
    return null;
  }
  if (typeof container.Labels === 'object') {
    return container.Labels[key] || null;
  }
  return null;
}

function getRepoName(repoUrl) {
  if (!repoUrl) {
    return null;
  }
  var raw = String(repoUrl).trim();
  if (!raw) {
    return null;
  }
  var name = null;
  if (raw.startsWith('http://') || raw.startsWith('https://')) {
    try {
      var url = new URL(raw);
      var path = url.pathname.replace(/\/+$/, '');
      var segments = path.split('/').filter(Boolean);
      if (segments.length > 0) {
        name = segments[segments.length - 1];
      }
    } catch (err) {
      name = null;
    }
  } else {
    var withoutQuery = raw.split('?')[0].replace(/\/+$/, '');
    var afterColon = withoutQuery.indexOf(':') !== -1
      ? withoutQuery.split(':').slice(1).join(':')
      : withoutQuery;
    var parts = afterColon.split('/').filter(Boolean);
    if (parts.length > 0) {
      name = parts[parts.length - 1];
    }
  }
  if (!name) {
    return null;
  }
  name = name.replace(/\.git$/i, '');
  return name || null;
}

function sanitizeRepoDirName(name) {
  if (!name) {
    return null;
  }
  var sanitized = String(name).replace(/[^A-Za-z0-9._-]/g, '-');
  if (!sanitized) {
    return null;
  }
  return sanitized.slice(0, 80);
}

function randomAnimal() {
  return ANIMALS[Math.floor(Math.random() * ANIMALS.length)];
}

function stripJsonComments(input) {
  var output = '';
  var inString = false;
  var inLineComment = false;
  var inBlockComment = false;
  var escapeNext = false;
  for (var i = 0; i < input.length; i += 1) {
    var char = input[i];
    var next = i + 1 < input.length ? input[i + 1] : '';

    if (inLineComment) {
      if (char === '\n') {
        inLineComment = false;
        output += char;
      }
      continue;
    }

    if (inBlockComment) {
      if (char === '*' && next === '/') {
        inBlockComment = false;
        i += 1;
      }
      continue;
    }

    if (inString) {
      output += char;
      if (escapeNext) {
        escapeNext = false;
        continue;
      }
      if (char === '\\') {
        escapeNext = true;
        continue;
      }
      if (char === '"') {
        inString = false;
      }
      continue;
    }

    if (char === '/' && next === '/') {
      inLineComment = true;
      i += 1;
      continue;
    }

    if (char === '/' && next === '*') {
      inBlockComment = true;
      i += 1;
      continue;
    }

    if (char === '"') {
      inString = true;
    }

    output += char;
  }

  return output;
}

function parseJsonc(contents) {
  try {
    return JSON.parse(contents);
  } catch (err) {
    try {
      return JSON.parse(stripJsonComments(contents));
    } catch (err2) {
      return null;
    }
  }
}

function resolveDevcontainerPath(repoDir, jsonDir, inputPath) {
  var raw = inputPath || '.';
  var normalized = String(raw);
  if (path.isAbsolute(normalized)) {
    normalized = normalized.replace(/^[/\\]+/, '');
  }
  return path.join(repoDir, jsonDir, normalized);
}

async function podmanImageExists(tag) {
  try {
    await runPodman(['image', 'exists', tag]);
    return true;
  } catch (err) {
    return false;
  }
}

function createDevcontainerTag(repoUrl, repoCommit, jsonPath, buildConfig) {
  var key = JSON.stringify({
    repoUrl: repoUrl || null,
    repoCommit: repoCommit || null,
    jsonPath: jsonPath || null,
    build: buildConfig || null
  });
  var digest = crypto.createHash('sha256').update(key).digest('hex').slice(0, 12);
  return 'uou-devcontainer-' + digest;
}

async function buildDevcontainerImage(repoDir, jsonDir, buildConfig, metadata) {
  if (!buildConfig) {
    return null;
  }
  var dockerfile = null;
  var contextDir = '.';
  if (typeof buildConfig === 'string') {
    dockerfile = buildConfig;
  } else if (typeof buildConfig === 'object') {
    dockerfile = buildConfig.dockerfile || 'Dockerfile';
    contextDir = buildConfig.context || '.';
  }
  if (!dockerfile) {
    return null;
  }
  var dockerfilePath = resolveDevcontainerPath(repoDir, jsonDir, dockerfile);
  var contextPath = resolveDevcontainerPath(repoDir, jsonDir, contextDir);
  var tag = createDevcontainerTag(
    metadata && metadata.repoUrl,
    metadata && metadata.repoCommit,
    metadata && metadata.jsonPath,
    buildConfig
  );
  if (await podmanImageExists(tag)) {
    return tag;
  }
  var labels = [
    'com.urban-octo-umbrella.devcontainer_cache=true'
  ];
  if (metadata && metadata.repoUrl) {
    labels.push('com.urban-octo-umbrella.repo_url=' + metadata.repoUrl);
  }
  if (metadata && metadata.repoCommit) {
    labels.push('com.urban-octo-umbrella.repo_commit=' + metadata.repoCommit);
  }
  if (metadata && metadata.jsonPath) {
    labels.push('com.urban-octo-umbrella.devcontainer_path=' + metadata.jsonPath);
  }
  labels.push('com.urban-octo-umbrella.built_at=' + new Date().toISOString());
  var buildArgs = ['build', '-t', tag];
  labels.forEach(function(label) {
    buildArgs.push('--label', label);
  });
  buildArgs.push('-f', dockerfilePath, contextPath);
  await runPodman(buildArgs);
  return tag;
}

async function findDevcontainerImage(repoUrl, accessToken) {
  if (!repoUrl) {
    return null;
  }
  var tempBase = await fsPromises.mkdtemp(path.join(os.tmpdir(), 'uou-devcontainer-'));
  var repoDir = path.join(tempBase, 'repo');
  try {
    var cloneArgs = ['clone', '--depth', '1', repoUrl, repoDir];
    if (accessToken) {
      var authHeader = Buffer.from('x-access-token:' + accessToken).toString('base64');
      cloneArgs = [
        '-c',
        'http.extraheader=AUTHORIZATION: basic ' + authHeader
      ].concat(cloneArgs);
    }
    await runGit(cloneArgs, { cwd: tempBase });

    var lsOutput = await runGit([
      '-C',
      repoDir,
      'ls-tree',
      '-r',
      '--name-only',
      'HEAD',
      '.devcontainer/devcontainer.json',
      'devcontainer.json'
    ]);
    var files = String(lsOutput || '')
      .split('\n')
      .map(function(line) { return line.trim(); })
      .filter(Boolean);
    var jsonPath = files.indexOf('.devcontainer/devcontainer.json') !== -1
      ? '.devcontainer/devcontainer.json'
      : (files.indexOf('devcontainer.json') !== -1 ? 'devcontainer.json' : null);
    if (!jsonPath) {
      return null;
    }
    var jsonContents = await runGit(['-C', repoDir, 'show', 'HEAD:' + jsonPath]);
    var parsed = parseJsonc(String(jsonContents || ''));
    if (!parsed) {
      return null;
    }
    if (typeof parsed.image === 'string') {
      var image = parsed.image.trim();
      return image || null;
    }
    var jsonDir = path.posix.dirname(jsonPath);
    var repoCommit = null;
    try {
      repoCommit = String(await runGit(['-C', repoDir, 'rev-parse', 'HEAD'])).trim();
    } catch (_) {
      repoCommit = null;
    }
    var builtImage = await buildDevcontainerImage(repoDir, jsonDir, parsed.build, {
      repoUrl: repoUrl,
      repoCommit: repoCommit,
      jsonPath: jsonPath
    });
    return builtImage || null;
  } catch (err) {
    console.warn('Failed to inspect devcontainer.json:', err.message || err);
    return null;
  } finally {
    try {
      await fsPromises.rm(tempBase, { recursive: true, force: true });
    } catch (_) {
      // ignore cleanup failure
    }
  }
}

async function getUsedTunnelNames() {
  try {
    var stdout = await runPodman([
      'ps',
      '--filter',
      'label=' + APP_LABEL,
      '--format',
      'json'
    ]);
    var containers = JSON.parse(stdout || '[]');
    return new Set(containers.map(function(container) {
      return getContainerLabel(container, TUNNEL_NAME_LABEL);
    }).filter(Boolean));
  } catch (err) {
    return new Set();
  }
}

async function generateTunnelName() {
  var used = await getUsedTunnelNames();
  for (var attempt = 0; attempt < 15; attempt += 1) {
    var candidate = TUNNEL_NAME_PREFIX + randomAnimal();
    if (!used.has(candidate)) {
      return candidate;
    }
  }
  return TUNNEL_NAME_PREFIX + randomAnimal() + '-' + crypto.randomBytes(2).toString('hex');
}

function isContainerRunning(container) {
  if (!container) {
    return false;
  }
  if (container.State) {
    return String(container.State).toLowerCase() === 'running';
  }
  var status = String(container.Status || '').toLowerCase();
  return status.startsWith('up');
}

function normalizeWorkspacePath(repoPath) {
  if (!repoPath) {
    return null;
  }
  var trimmed = String(repoPath).trim();
  if (!trimmed) {
    return null;
  }
  return trimmed.startsWith('/') ? trimmed : '/' + trimmed;
}

function buildTunnelUrls(tunnelName, repoPath) {
  if (!tunnelName) {
    return { webUrl: null, vscodeUri: null };
  }
  var normalizedPath = normalizeWorkspacePath(repoPath);
  var webBase = TUNNEL_WEB_BASE.replace(/\/+$/, '');
  var webUrl = webBase + '/' + encodeURIComponent(tunnelName);
  var vscodeUri = 'vscode://vscode-remote/tunnel+' + encodeURIComponent(tunnelName);
  if (normalizedPath) {
    var encodedPath = encodeURI(normalizedPath);
    webUrl += encodedPath;
    vscodeUri += encodedPath;
  }
  return { webUrl: webUrl, vscodeUri: vscodeUri };
}

async function getContainerRunningState(containerId) {
  try {
    var stdout = await runPodman(['inspect', '-f', '{{.State.Running}}', containerId]);
    return String(stdout || '').trim() === 'true';
  } catch (err) {
    return false;
  }
}

async function getContainerLogs(containerId) {
  try {
    var stdout = await runPodman(['logs', '--tail', '200', containerId]);
    return String(stdout || '').trim();
  } catch (err) {
    return null;
  }
}

async function getContainerExitDetails(containerId) {
  try {
    var stdout = await runPodman([
      'inspect',
      '-f',
      '{{.State.ExitCode}} {{.State.Error}}',
      containerId
    ]);
    return String(stdout || '').trim();
  } catch (err) {
    return null;
  }
}

async function ensureHostDir(dirPath) {
  if (!dirPath) {
    return;
  }
  await fsPromises.mkdir(dirPath, { recursive: true });
}

router.get('/', async function(req, res) {
  try {
    var stdout = await runPodman([
      'ps',
      '-a',
      '--filter',
      'label=com.urban-octo-umbrella.managed=true',
      '--format',
      'json'
    ]);
    var containers = JSON.parse(stdout || '[]');
    containers.sort(function(a, b) {
      var aTime = Date.parse(a.CreatedAt || '') || 0;
      var bTime = Date.parse(b.CreatedAt || '') || 0;
      return bTime - aTime;
    });
    containers = containers.slice(0, MAX_ACTIVE_CONTAINERS);
    var normalized = await Promise.all(containers.map(async function(container) {
      var tunnelName = getContainerLabel(container, TUNNEL_NAME_LABEL);
      var repoPath = getContainerLabel(container, REPO_PATH_LABEL);
      var running = isContainerRunning(container);
      var urls = running ? buildTunnelUrls(tunnelName, repoPath) : { webUrl: null, vscodeUri: null };
      return {
        id: container.Id,
        name: Array.isArray(container.Names) ? container.Names[0] : container.Names,
        image: container.Image,
        status: container.Status,
        createdAt: container.CreatedAt,
        tunnel: tunnelName ? {
          name: tunnelName,
          status: running ? 'running' : 'stopped',
          webUrl: urls.webUrl,
          vscodeUri: urls.vscodeUri
        } : null,
        repoPath: repoPath
      };
    }));
    res.json({ ok: true, containers: normalized });
  } catch (err) {
    res.status(500).json({
      ok: false,
      error: 'Failed to list containers',
      details: err.stderr ? String(err.stderr).trim() : String(err.message || err)
    });
  }
});

router.post('/', async function(req, res) {
  try {
    if (!HOST_TUNNEL_DATA_DIR && !TUNNEL_ACCESS_TOKEN && !TUNNEL_REFRESH_TOKEN) {
      return res.status(400).json({
        ok: false,
        error: 'Missing CODE_TUNNEL_DATA_DIR_HOST or CODE_TUNNEL_ACCESS_TOKEN/CODE_TUNNEL_REFRESH_TOKEN for tunnel auth'
      });
    }
    if (HOST_TUNNEL_DATA_DIR) {
      await ensureHostDir(HOST_TUNNEL_DATA_DIR);
    }
    var repoUrl = req.body && req.body.repoUrl ? String(req.body.repoUrl).trim() : '';
    if (repoUrl === '') {
      repoUrl = null;
    }
    if (repoUrl && !(req.user && req.user.accessToken) && String(process.env.NO_AUTH || '').toLowerCase() !== 'true') {
      return res.status(400).json({ ok: false, error: 'GitHub token missing for private repo access' });
    }
    var repoName = repoUrl ? getRepoName(repoUrl) : null;
    if (repoUrl && !repoName) {
      return res.status(400).json({ ok: false, error: 'Invalid repository URL' });
    }
    var repoDirName = repoName ? sanitizeRepoDirName(repoName) : null;
    var repoDir = repoDirName ? ('/root/workspace/' + repoDirName) : null;
    var accessToken = req.user && req.user.accessToken ? String(req.user.accessToken) : null;
    var devcontainerImage = repoUrl ? await findDevcontainerImage(repoUrl, accessToken) : null;
    var imageToUse = devcontainerImage || DEFAULT_IMAGE;

    var tunnelName = await generateTunnelName();
    var name = tunnelName;
    var args = [
      'run',
      '-d',
      '--name',
      name,
      '--hostname',
      tunnelName,
      '--label',
      APP_LABEL,
      '--label',
      TUNNEL_NAME_LABEL + '=' + tunnelName
    ];
    if (HOST_TUNNEL_DATA_DIR) {
      args.push('-v', HOST_TUNNEL_DATA_DIR + ':' + CONTAINER_TUNNEL_DATA_DIR);
    }
    if (TUNNEL_ACCESS_TOKEN) {
      args.push('-e', 'VSCODE_CLI_ACCESS_TOKEN=' + TUNNEL_ACCESS_TOKEN);
    }
    if (TUNNEL_REFRESH_TOKEN) {
      args.push('-e', 'VSCODE_CLI_REFRESH_TOKEN=' + TUNNEL_REFRESH_TOKEN);
    }
    if (repoUrl && repoDir) {
      args.push('--label', REPO_URL_LABEL + '=' + repoUrl);
      args.push('--label', REPO_PATH_LABEL + '=' + repoDir);
    }
    args = args.concat([
      '--pull=missing',
      imageToUse,
      'sh',
      '-c',
      'set -e; ' +
        'mkdir -p "' + CONTAINER_TUNNEL_DATA_DIR + '"; ' +
        'export VSCODE_CLI_DATA_DIR="' + CONTAINER_TUNNEL_DATA_DIR + '"; ' +
        'code_bin="/opt/vscode-cli/code"; ' +
        'if [ ! -x "$code_bin" ] && [ -x "/opt/vscode-cli/bin/code" ]; then ' +
          'code_bin="/opt/vscode-cli/bin/code"; ' +
        'fi; ' +
        'if [ ! -x "$code_bin" ]; then ' +
          'if ! command -v curl >/dev/null 2>&1 || ! command -v tar >/dev/null 2>&1; then ' +
            'if command -v apt-get >/dev/null 2>&1; then ' +
              'apt-get update; ' +
              'apt-get install -y curl ca-certificates tar; ' +
            'elif command -v apk >/dev/null 2>&1; then ' +
              'apk add --no-cache curl ca-certificates tar; ' +
            'elif command -v dnf >/dev/null 2>&1; then ' +
              'dnf -y install curl ca-certificates tar; ' +
            'elif command -v yum >/dev/null 2>&1; then ' +
              'yum -y install curl ca-certificates tar; ' +
            'fi; ' +
          'fi; ' +
          'if ! command -v curl >/dev/null 2>&1 || ! command -v tar >/dev/null 2>&1; then ' +
            'echo "curl and tar are required to install the VS Code CLI." >&2; ' +
            'exit 1; ' +
          'fi; ' +
          'arch=$(uname -m); ' +
          'case "$arch" in ' +
            'x86_64) platform=alpine-x64 ;; ' +
            'aarch64|arm64) platform=alpine-arm64 ;; ' +
            'armv7l) platform=alpine-armhf ;; ' +
            '*) echo "Unsupported CPU architecture: $arch" >&2; exit 1 ;; ' +
          'esac; ' +
          'mkdir -p /opt/vscode-cli; ' +
          'curl -fSL "https://code.visualstudio.com/sha/download?build=stable&os=cli-$platform" -o /tmp/vscode-cli.tar.gz; ' +
          'tar -xzf /tmp/vscode-cli.tar.gz -C /opt/vscode-cli; ' +
          'rm -f /tmp/vscode-cli.tar.gz; ' +
          'chmod +x /opt/vscode-cli/code /opt/vscode-cli/bin/code 2>/dev/null || true; ' +
          'if [ -x "/opt/vscode-cli/code" ]; then ' +
            'code_bin="/opt/vscode-cli/code"; ' +
          'elif [ -x "/opt/vscode-cli/bin/code" ]; then ' +
            'code_bin="/opt/vscode-cli/bin/code"; ' +
          'fi; ' +
        'fi; ' +
        'if [ ! -x "$code_bin" ]; then ' +
          'echo "VS Code CLI (code) could not be installed." >&2; ' +
          'exit 1; ' +
        'fi; ' +
        '"$code_bin" tunnel --accept-server-license-terms --name "' + tunnelName + '"' +
          (TUNNEL_DISABLE_TELEMETRY ? ' --disable-telemetry' : '') +
          ' --log info'
    ]);
    var stdout = await runPodman(args);
    var id = String(stdout || '').trim();
    var running = await getContainerRunningState(id);
    if (!running) {
      var logs = await getContainerLogs(id);
      var exitDetails = await getContainerExitDetails(id);
      if (!KEEP_FAILED_CONTAINERS) {
        try {
          await runPodman(['rm', '-f', id]);
        } catch (_) {
          // ignore cleanup failure
        }
      }
      return res.status(500).json({
        ok: false,
        error: 'Container failed to start tunnel',
        details: logs || exitDetails || 'Container exited during startup',
        id: KEEP_FAILED_CONTAINERS ? id : undefined
      });
    }
    if (repoUrl && repoDir) {
      var stillRunning = await getContainerRunningState(id);
      if (!stillRunning) {
        var stoppedLogs = await getContainerLogs(id);
        var stoppedExit = await getContainerExitDetails(id);
        if (!KEEP_FAILED_CONTAINERS) {
          try {
            await runPodman(['rm', '-f', id]);
          } catch (_) {
            // ignore cleanup failure
          }
        }
        return res.status(500).json({
          ok: false,
          error: 'Container stopped before repo clone',
          details: stoppedLogs || stoppedExit || 'Container exited during startup',
          id: KEEP_FAILED_CONTAINERS ? id : undefined
        });
      }
      try {
        await runPodman(['exec', id, 'mkdir', '-p', '/root/workspace']);
        if (accessToken) {
          var authHeader = Buffer.from('x-access-token:' + accessToken).toString('base64');
          await runPodman([
            'exec',
            '-e',
            'GIT_TERMINAL_PROMPT=0',
            id,
            'git',
            '-c',
            'http.extraheader=AUTHORIZATION: basic ' + authHeader,
            'clone',
            repoUrl,
            repoDir
          ]);
        } else {
          await runPodman([
            'exec',
            '-e',
            'GIT_TERMINAL_PROMPT=0',
            id,
            'git',
            'clone',
            repoUrl,
            repoDir
          ]);
        }
      } catch (cloneErr) {
        try {
          await runPodman(['rm', '-f', id]);
        } catch (_) {
          // ignore cleanup failure
        }
        throw cloneErr;
      }
    }
    res.json({ ok: true, id: id, name: name });
  } catch (err) {
    res.status(500).json({
      ok: false,
      error: 'Failed to start container',
      details: err.stderr ? String(err.stderr).trim() : String(err.message || err)
    });
  }
});

router.post('/:id/start', async function(req, res) {
  var id = req.params.id;
  try {
    await runPodman(['start', id]);
    res.json({ ok: true, id: id });
  } catch (err) {
    res.status(500).json({
      ok: false,
      error: 'Failed to start container',
      details: err.stderr ? String(err.stderr).trim() : String(err.message || err)
    });
  }
});

router.post('/:id/stop', async function(req, res) {
  var id = req.params.id;
  try {
    await runPodman(['stop', id]);
    res.json({ ok: true, id: id });
  } catch (err) {
    res.status(500).json({
      ok: false,
      error: 'Failed to stop container',
      details: err.stderr ? String(err.stderr).trim() : String(err.message || err)
    });
  }
});

router.delete('/:id', async function(req, res) {
  var id = req.params.id;
  try {
    await runPodman(['rm', '-f', id]);
    res.json({ ok: true, id: id });
  } catch (err) {
    res.status(500).json({
      ok: false,
      error: 'Failed to delete container',
      details: err.stderr ? String(err.stderr).trim() : String(err.message || err)
    });
  }
});

module.exports = router;
