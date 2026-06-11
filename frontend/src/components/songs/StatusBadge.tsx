import type {SongStatus} from '../../lib/types';

const styles: Record<SongStatus, {label: string; className: string}> = {
  not_learned: {label: 'Not learned', className: 'badge-ghost'},
  learning: {label: 'Learning', className: 'badge-warning'},
  learned: {label: 'Learned', className: 'badge-info'},
  nailed: {label: 'Nailed!', className: 'badge-success'},
};

export default function StatusBadge({status}: {status: SongStatus}) {
  const s = styles[status] ?? styles.not_learned;
  return <span className={`badge ${s.className}`}>{s.label}</span>;
}
