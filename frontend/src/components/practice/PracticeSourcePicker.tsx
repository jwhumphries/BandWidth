import {useBandFolders} from '../../hooks/bandfolders';
import {useBands} from '../../hooks/bands';
import {useFolders} from '../../hooks/folders';
import type {BandSummary} from '../../lib/types';

function BandOptgroup({band}: {band: BandSummary}) {
  const {data: folders = []} = useBandFolders(band.id);
  return (
    <optgroup label={band.name}>
      <option value={`band:${band.id}`}>All {band.name} songs</option>
      {folders.map(f => (
        <option key={f.id} value={`bandfolder:${band.id}:${f.id}`}>
          {f.name}
        </option>
      ))}
    </optgroup>
  );
}

export default function PracticeSourcePicker({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) {
  const {data: folders = []} = useFolders();
  const {data: bands = []} = useBands();
  return (
    <select
      className="select"
      aria-label="Source"
      value={value}
      onChange={e => onChange(e.target.value)}
    >
      <option value="all">All Songs</option>
      {folders.length > 0 && (
        <optgroup label="Folders">
          {folders.map(f => (
            <option key={f.id} value={`folder:${f.id}`}>
              {f.name}
            </option>
          ))}
        </optgroup>
      )}
      {bands.map(band => (
        <BandOptgroup key={band.id} band={band} />
      ))}
    </select>
  );
}
