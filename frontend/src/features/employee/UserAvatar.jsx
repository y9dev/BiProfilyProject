// Thin wrapper over the shared Avatar (src/avatar/Avatar.jsx, props: universe, gender, style, size).
// Resolved with import.meta.glob so the screen still renders an initials badge if that file is absent.
import { initials } from './format'

const found = import.meta.glob('../../avatar/Avatar.jsx', { eager: true })
const SharedAvatar = Object.values(found)[0]?.default

export default function UserAvatar({ avatar, name, size = 72 }) {
  if (SharedAvatar && avatar) {
    return <SharedAvatar universe={avatar.universe} gender={avatar.gender} style={avatar.style} size={size} />
  }
  return (
    <span
      aria-hidden="true"
      style={{ width: size, height: size, fontSize: size * 0.36 }}
      className="inline-flex shrink-0 items-center justify-center rounded-full bg-primary-soft font-bold text-primary"
    >
      {initials(name)}
    </span>
  )
}
