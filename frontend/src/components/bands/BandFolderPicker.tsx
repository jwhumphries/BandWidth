import {useBandFolders, useSetBandFolderEntries} from '../../hooks/bandfolders';

export default function BandFolderPicker({
  bandId,
  songId,
  canEdit,
}: {
  bandId: number;
  songId: number;
  canEdit: boolean;
}) {
  const {data: folders = []} = useBandFolders(bandId);
  const setEntries = useSetBandFolderEntries(bandId);

  if (folders.length === 0) {
    return (
      <p className="text-base-content/60 text-sm">
        No folders yet — create one from the band page.
      </p>
    );
  }

  const toggle = (folderId: number, member: boolean) => {
    const folder = folders.find(f => f.id === folderId);
    if (!folder) return;
    const songIds = member
      ? [...folder.songIds, songId]
      : folder.songIds.filter(id => id !== songId);
    setEntries.mutate({id: folderId, songIds});
  };

  return (
    <ul className="flex flex-col gap-1">
      {folders.map(f => {
        const member = f.songIds.includes(songId);
        return (
          <li key={f.id}>
            <label className="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                className="checkbox checkbox-sm"
                checked={member}
                disabled={!canEdit}
                onChange={() => toggle(f.id, !member)}
                aria-label={f.name}
              />
              <span>{f.name}</span>
            </label>
          </li>
        );
      })}
    </ul>
  );
}
