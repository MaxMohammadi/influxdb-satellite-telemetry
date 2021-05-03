// Libraries
import React, {FC} from 'react'

// Components
import {EmptyState, ComponentSize} from '@influxdata/clockface'
import AddResourceDropdown from 'src/shared/components/AddResourceDropdown'

// Actions
import {createDashboard} from 'src/dashboards/actions/thunks'

interface ComponentProps {
  searchTerm?: string
  onCreateDashboard: typeof createDashboard
  summonImportOverlay: () => void
  summonImportFromTemplateOverlay: () => void
}

const DashboardsTableEmpty: FC<ComponentProps> = ({
  searchTerm,
  onCreateDashboard,
  summonImportOverlay,
  summonImportFromTemplateOverlay,
}) => {
  if (searchTerm) {
    return (
      <EmptyState size={ComponentSize.Large} testID="empty-dashboards-list">
        <EmptyState.Text>No Dashboards match your search term</EmptyState.Text>
      </EmptyState>
    )
  }

  return (
    <EmptyState size={ComponentSize.Large} testID="empty-dashboards-list">
      <EmptyState.Text>
        Looks like you don't have any <b>Dashboards</b>, why not create one?
      </EmptyState.Text>
      <AddResourceDropdown
        onSelectNew={onCreateDashboard}
        onSelectImport={summonImportOverlay}
        onSelectTemplate={summonImportFromTemplateOverlay}
        resourceName="Dashboard"
        canImportFromTemplate={true}
      />
    </EmptyState>
  )
}

export default DashboardsTableEmpty
