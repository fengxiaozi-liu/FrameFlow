let active: HTMLMediaElement | undefined;
/** One preview at a time, including detached Audio instances used for dialogue. */
export function claimPlayback(media: HTMLMediaElement) {
  if (active && active !== media) active.pause();
  document.querySelectorAll<HTMLMediaElement>('audio,video').forEach(other => {
    if (other !== media && !other.paused) other.pause();
  });
  active = media;
}
export function stopPlayback() { active?.pause(); active = undefined; }
