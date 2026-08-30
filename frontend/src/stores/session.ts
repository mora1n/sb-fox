import { useNodeGroupsStore } from './nodeGroups'
import { useNodesStore } from './nodes'
import { useProfilesStore } from './profiles'
import { usePublicTokenStore } from './publicToken'
import { useRuleSetsStore } from './ruleSets'
import { useSettingsStore } from './settings'
import { useSourcesStore } from './sources'
import { useTemplatesStore } from './templates'
import { useUsersStore } from './users'

export function resetSessionStores(): void {
  useTemplatesStore().reset()
  useNodesStore().reset()
  useNodeGroupsStore().reset()
  useProfilesStore().reset()
  usePublicTokenStore().reset()
  useRuleSetsStore().reset()
  useSourcesStore().reset()
  useUsersStore().reset()
  useSettingsStore().resetSessionState()
}
