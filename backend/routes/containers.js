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
var SSH_USER = 'root';
var SSH_BIND = process.env.SSH_BIND || '0.0.0.0';
var DOMAIN_NAME = process.env.DOMAIN_NAME || 'localhost';
var PASSWORD_LABEL = 'com.urban-octo-umbrella.ssh_password';
var SSH_NAME_LABEL = 'com.urban-octo-umbrella.ssh_name';
var REPO_URL_LABEL = 'com.urban-octo-umbrella.repo_url';
var REPO_PATH_LABEL = 'com.urban-octo-umbrella.repo_path';
var DEFAULT_IMAGE = 'mcr.microsoft.com/devcontainers/universal:latest';

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

function getHostForSsh(req) {
  return process.env.SSH_HOST || DOMAIN_NAME || req.hostname || 'localhost';
}

async function getSshPort(containerId, containerPort) {
  try {
    var stdout = await runPodman(['port', containerId, containerPort + '/tcp']);
    var line = String(stdout || '').trim().split('\n')[0] || '';
    var match = line.match(/:(\d+)\s*$/);
    if (!match) {
      return null;
    }
    return match[1];
  } catch (err) {
    return null;
  }
}

function getContainerLabel(container, key) {
  if (!container || !container.Labels) {
    return null;
  }
  if (typeof container.Labels === 'object') {
    return container.Labels[key] || null;
  }
  return null;
}

function generatePassword() {
  return crypto.randomBytes(16).toString('hex');
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

async function getUsedSshNames() {
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
      return getContainerLabel(container, SSH_NAME_LABEL);
    }).filter(Boolean));
  } catch (err) {
    return new Set();
  }
}

async function generateSshName() {
  var used = await getUsedSshNames();
  for (var attempt = 0; attempt < 15; attempt += 1) {
    var candidate = randomAnimal();
    if (!used.has(candidate)) {
      return candidate;
    }
  }
  return randomAnimal() + '-' + crypto.randomBytes(2).toString('hex');
}

router.get('/', async function(req, res) {
  try {
    var stdout = await runPodman([
      'ps',
      '--filter',
      'label=com.urban-octo-umbrella.managed=true',
      '--format',
      'json'
    ]);
    var containers = JSON.parse(stdout || '[]');
    var normalized = await Promise.all(containers.map(async function(container) {
      var containerPort = '2222';
      var sshPort = await getSshPort(container.Id, containerPort);
      var sshHost = sshPort ? getHostForSsh(req) : null;
      var sshUser = SSH_USER;
      var includePort = true;
      var sshTarget = sshPort
        ? (sshUser + '@' + sshHost + (includePort ? ':' + sshPort : ''))
        : null;
      var repoPath = getContainerLabel(container, REPO_PATH_LABEL);
      var vscodeUri = null;
      if (sshTarget) {
        var base = 'vscode://vscode-remote/ssh-remote+' + encodeURIComponent(sshTarget);
        vscodeUri = repoPath ? base + encodeURI(repoPath) : base;
      }
      return {
        id: container.Id,
        name: Array.isArray(container.Names) ? container.Names[0] : container.Names,
        image: container.Image,
        status: container.Status,
        createdAt: container.CreatedAt,
        ssh: sshPort ? {
          host: sshHost,
          port: sshPort,
          user: sshUser,
          password: getContainerLabel(container, PASSWORD_LABEL),
          vscodeUri: vscodeUri
        } : null
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

    var sshName = await generateSshName();
    var name = sshName;
    var password = generatePassword();
    var sshdPort = '2222';
    var args = [
      'run',
      '-d',
      '--name',
      name,
      '--hostname',
      sshName,
      '--label',
      APP_LABEL,
      '--label',
      PASSWORD_LABEL + '=' + password,
      '--label',
      SSH_NAME_LABEL + '=' + sshName,
    ];
    if (repoUrl && repoDir) {
      args.push('--label', REPO_URL_LABEL + '=' + repoUrl);
      args.push('--label', REPO_PATH_LABEL + '=' + repoDir);
    }
    args.push('--publish', SSH_BIND + '::' + sshdPort);
    args = args.concat([
      '--pull=missing',
      imageToUse,
      'sh',
      '-c',
      'set -e; ' +
        'if ! command -v sshd >/dev/null 2>&1; then ' +
          'if command -v apt-get >/dev/null 2>&1; then ' +
            'apt-get update && apt-get install -y openssh-server; ' +
          'elif command -v apk >/dev/null 2>&1; then ' +
            'apk add --no-cache openssh-server; ' +
          'elif command -v dnf >/dev/null 2>&1; then ' +
            'dnf -y install openssh-server; ' +
          'elif command -v yum >/dev/null 2>&1; then ' +
            'yum -y install openssh-server; ' +
          'fi; ' +
        'fi; ' +
        'if command -v sshd >/dev/null 2>&1; then ' +
          'echo \'' + SSH_USER + ':' + password + '\' | chpasswd; ' +
          'mkdir -p /etc/ssh/sshd_config.d; ' +
          'cat > /etc/ssh/sshd_config.d/99-uou.conf <<EOF\n' +
          'Port ' + sshdPort + '\n' +
          'PasswordAuthentication yes\n' +
          'KbdInteractiveAuthentication yes\n' +
          'ChallengeResponseAuthentication yes\n' +
          'PermitRootLogin yes\n' +
          'UsePAM yes\n' +
          'EOF\n' +
          'mkdir -p /var/run/sshd; ' +
          'ssh-keygen -A; ' +
          '/usr/sbin/sshd -D -e -p ' + sshdPort + '; ' +
        'else ' +
          'sleep 3600; ' +
        'fi'
    ]);
    var stdout = await runPodman(args);
    var id = String(stdout || '').trim();
    if (repoUrl && repoDir) {
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
