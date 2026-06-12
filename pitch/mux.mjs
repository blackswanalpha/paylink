/**
 * Builds the final narrated demo: transcodes the silent webm to H.264 (trimming the
 * boot-ivory lead-in so it opens on the title), and drops each vo/<beat>.mp3 at the
 * timestamp its beat appeared on screen (from vo/marks.json). Pure ffmpeg, one pass.
 */
import { readFileSync, readdirSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

const ROOT = '/home/mbugua/Documents/augment-projects/linkMint/pitch';
const OFFSET = 0.25; // start each clip a touch after its beat appears (a breath)

const marks = JSON.parse(readFileSync(`${ROOT}/vo/marks.json`, 'utf8'));
const webm = readdirSync(`${ROOT}/video`).find((f) => f.endsWith('.webm'));
if (!webm) throw new Error('no webm in video/');

const frontTrim = marks.find((m) => m.beat === 't1_title').t; // anchor: title fully shown
const beats = marks.map((m) => ({ ...m, pos: Math.max(0, m.t - frontTrim + OFFSET) }));

// inputs: [0] = video (input-seek to frontTrim), [1..N] = the beat mp3s in order
const inputs = ['-ss', String(frontTrim), '-i', `${ROOT}/video/${webm}`];
beats.forEach((b) => inputs.push('-i', `${ROOT}/vo/${b.beat}.mp3`));

const fc = [];
fc.push('[0:v]fps=30,scale=1280:720:flags=lanczos[v]');
beats.forEach((b, i) => {
  const ms = Math.round(b.pos * 1000);
  fc.push(`[${i + 1}]adelay=${ms}:all=1[a${i + 1}]`);
});
const aLabels = beats.map((_, i) => `[a${i + 1}]`).join('');
fc.push(`${aLabels}amix=inputs=${beats.length}:normalize=0:dropout_transition=0[amx]`);
fc.push('[amx]loudnorm=I=-16:TP=-1.5:LRA=11,aresample=48000,aformat=channel_layouts=stereo[mix]');

const args = [
  '-y', ...inputs,
  '-filter_complex', fc.join(';'),
  '-map', '[v]', '-map', '[mix]',
  '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-crf', '20', '-preset', 'slow',
  '-c:a', 'aac', '-b:a', '160k',
  '-movflags', '+faststart',
  `${ROOT}/linkmint-demo.mp4`,
];

console.log('frontTrim', frontTrim.toFixed(2), 's   clip positions:');
beats.forEach((b) => console.log('  ', b.beat.padEnd(12), b.pos.toFixed(2), 's'));
execFileSync('ffmpeg', args, { stdio: ['ignore', 'inherit', 'inherit'] });
console.log('MUXED -> linkmint-demo.mp4');
