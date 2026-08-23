import FolderSidebarView from '../folders/FolderSidebarView';
import {
  useBandFolders,
  useCreateBandFolder,
  useDeleteBandFolder,
  useRenameBandFolder,
  useReorderBandFolders,
} from '../../hooks/bandfolders';

/** The band view's folder rail. Viewers get the list without the controls. */
export default function BandFolderSidebar({
  bandId,
  canEdit,
  selectedId,
  onSelect,
}: {
  bandId: number;
  canEdit: boolean;
  selectedId: number | null;
  onSelect: (id: number | null) => void;
}) {
  const {data: folders = []} = useBandFolders(bandId);
  const createFolder = useCreateBandFolder(bandId);
  const renameFolder = useRenameBandFolder(bandId);
  const deleteFolder = useDeleteBandFolder(bandId);
  const reorderFolders = useReorderBandFolders(bandId);

  return (
    <FolderSidebarView
      folders={folders}
      canEdit={canEdit}
      selectedId={selectedId}
      creating={createFolder.isPending}
      onSelect={onSelect}
      onCreate={name => createFolder.mutate({name})}
      onRename={(id, name) => renameFolder.mutate({id, name})}
      onDelete={folder =>
        deleteFolder.mutate(folder.id, {
          onSuccess: () => {
            if (selectedId === folder.id) onSelect(null);
          },
        })
      }
      onReorder={ids => reorderFolders.mutate(ids)}
    />
  );
}
