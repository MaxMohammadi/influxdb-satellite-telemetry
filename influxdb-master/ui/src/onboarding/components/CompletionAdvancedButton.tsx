// Libraries
import React, {PureComponent} from 'react'
import {withRouter, RouteComponentProps} from 'react-router-dom'
import {connect} from 'react-redux'

// Components
import {Button, ComponentColor, ComponentSize} from '@influxdata/clockface'
import {ErrorHandling} from 'src/shared/decorators/errors'

// Types
import {AppState} from 'src/types'

interface OwnProps {
  onExit: () => void
}

interface StateProps {
  orgID: string | null
}

type Props = OwnProps & StateProps & RouteComponentProps<{orgID: string}>

@ErrorHandling
class CompletionAdvancedButton extends PureComponent<Props> {
  public render() {
    return (
      <Button
        text="Advanced"
        color={ComponentColor.Success}
        size={ComponentSize.Large}
        onClick={this.handleAdvanced}
        testID="button--advanced"
      />
    )
  }

  private handleAdvanced = (): void => {
    const {history, orgID, onExit} = this.props

    if (orgID) {
      history.push(`/orgs/${orgID}/load-data/buckets`)
    } else {
      onExit()
    }
  }
}

const mstp = (state: AppState) => {
  return {
    orgID: state.onboarding.orgID,
  }
}
export default connect<StateProps>(mstp)(withRouter(CompletionAdvancedButton))
