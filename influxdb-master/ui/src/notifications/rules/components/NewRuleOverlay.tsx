// Libraries
import React, {useMemo, FC} from 'react'
import {withRouter, RouteComponentProps} from 'react-router-dom'
import {connect, ConnectedProps} from 'react-redux'

// Actions
import {createRule} from 'src/notifications/rules/actions/thunks'

// Components
import RuleOverlayContents from 'src/notifications/rules/components/RuleOverlayContents'
import {Overlay} from '@influxdata/clockface'

// Utils
import RuleOverlayProvider from 'src/notifications/rules/components/RuleOverlayProvider'
import {initRuleDraft} from 'src/notifications/rules/utils'

// Types
import {NotificationRuleDraft} from 'src/types'

type ReduxProps = ConnectedProps<typeof connector>
type Props = RouteComponentProps<{orgID: string}> & ReduxProps

const NewRuleOverlay: FC<Props> = ({
  match: {
    params: {orgID},
  },
  history,
  onCreateRule,
}) => {
  const handleDismiss = () => {
    history.push(`/orgs/${orgID}/alerting`)
  }

  const handleCreateRule = async (rule: NotificationRuleDraft) => {
    await onCreateRule(rule)

    handleDismiss()
  }

  const initialState = useMemo(() => initRuleDraft(orgID), [orgID])

  return (
    <RuleOverlayProvider initialState={initialState}>
      <Overlay visible={true}>
        <Overlay.Container maxWidth={800}>
          <Overlay.Header
            title="Create a Notification Rule"
            onDismiss={handleDismiss}
            testID="dismiss-overlay"
          />
          <Overlay.Body>
            <RuleOverlayContents
              saveButtonText="Create Notification Rule"
              onSave={handleCreateRule}
            />
          </Overlay.Body>
        </Overlay.Container>
      </Overlay>
    </RuleOverlayProvider>
  )
}

const mdtp = {
  onCreateRule: createRule,
}

const connector = connect(null, mdtp)

export default connector(withRouter(NewRuleOverlay))
