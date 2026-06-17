import type {SongStatus} from '../../lib/types';

const labels: Record<SongStatus, string> = {
  not_learned: 'Not learned',
  learning: 'Learning',
  learned: 'Learned',
  nailed: 'Nailed!',
};

export default function StatusBadge({status}: {status: SongStatus}) {
  const label = labels[status] ?? labels.not_learned;
  return <span className={`status-chip status-chip-${status}`}>{label}</span>;
}
