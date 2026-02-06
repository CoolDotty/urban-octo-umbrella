var express = require('express');
var router = express.Router();
var { execFile } = require('child_process');

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

router.get('/', async function(req, res) {
  try {
    var stdout = await runPodman(['ps', '--format', 'json']);
    var containers = JSON.parse(stdout || '[]');
    var normalized = containers.map(function(container) {
      return {
        id: container.Id,
        name: Array.isArray(container.Names) ? container.Names[0] : container.Names,
        image: container.Image,
        status: container.Status,
        createdAt: container.CreatedAt
      };
    });
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
  var name = 'uou-' + Date.now();
  try {
    var stdout = await runPodman([
      'run',
      '-d',
      '--name',
      name,
      '--pull=missing',
      'docker.io/library/alpine:latest',
      'sleep',
      '3600'
    ]);
    var id = String(stdout || '').trim();
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
