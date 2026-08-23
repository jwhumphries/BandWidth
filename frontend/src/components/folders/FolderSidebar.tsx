import FolderSidebarView from './FolderSidebarView';
import {
  useCreateFolder,
  useDeleteFolder,
  useFolders,
  useRenameFolder,
  useReorderFolders,
} from '../../hooks/folders';

/** The personal library's folder rail. */
export default function FolderSidebar({
  selectedId,
  onSelect,
}: {
  selectedId: number | null;
  onSelect: (id: number | null) => void;
}) {
  const {data: folders = []} = useFolders();
  const createFolder = useCreateFolder();
  const renameFolder = useRenameFolder();
  const deleteFolder = useDeleteFolder();
  const reorderFolders = useReorderFolders();

  // Deleting the selected folder falls back to all songs on its own: the folder
  // list refetches without it and useFolderSelection drops the stale id.
  return (
    <FolderSidebarView
      folders={folders}
      canEdit
      selectedId={selectedId}
      creating={createFolder.isPending}
      onSelect={onSelect}
      onCreate={name => createFolder.mutate({name})}
      onRename={(id, name) => renameFolder.mutate({id, name})}
      onDelete={folder => deleteFolder.mutate(folder.id)}
      onReorder={ids => reorderFolders.mutate(ids)}
    />
  );
}
