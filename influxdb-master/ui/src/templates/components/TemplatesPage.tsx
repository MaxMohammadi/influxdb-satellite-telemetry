// Libraries
import React, {PureComponent} from 'react'
import _ from 'lodash'
import {connect, ConnectedProps} from 'react-redux'

// Components
import FilterList from 'src/shared/components/FilterList'
import TemplatesList from 'src/templates/components/TemplatesList'
import StaticTemplatesList, {
  StaticTemplate,
  TemplateOrSummary,
} from 'src/templates/components/StaticTemplatesList'
import {ErrorHandling} from 'src/shared/decorators/errors'
import SearchWidget from 'src/shared/components/search_widget/SearchWidget'
import GetResources from 'src/resources/components/GetResources'
import TabbedPageHeader from 'src/shared/components/tabbed_page/TabbedPageHeader'
import ResourceSortDropdown from 'src/shared/components/resource_sort_dropdown/ResourceSortDropdown'

// Types
import {AppState, ResourceType, TemplateSummary} from 'src/types'
import {SortTypes} from 'src/shared/utils/sort'
import {
  Sort,
  Button,
  ComponentColor,
  IconFont,
  SelectGroup,
} from '@influxdata/clockface'
import {TemplateSortKey} from 'src/shared/components/resource_sort_dropdown/generateSortItems'
import {staticTemplates as statics} from 'src/templates/constants/defaultTemplates'

// Selectors
import {getAll} from 'src/resources/selectors/getAll'

// Constants
const staticTemplates: StaticTemplate[] = _.map(statics, (template, name) => ({
  name,
  template: template as TemplateOrSummary,
}))

interface OwnProps {
  onImport: () => void
}

type ReduxProps = ConnectedProps<typeof connector>
type Props = OwnProps & ReduxProps

interface State {
  searchTerm: string
  sortKey: TemplateSortKey
  sortDirection: Sort
  sortType: SortTypes
  activeTab: string
}

const FilterStaticTemplates = FilterList<StaticTemplate>()
const FilterTemplateSummaries = FilterList<TemplateSummary>()

@ErrorHandling
class TemplatesPage extends PureComponent<Props, State> {
  constructor(props) {
    super(props)

    this.state = {
      searchTerm: '',
      sortKey: 'meta.name',
      sortDirection: Sort.Ascending,
      sortType: SortTypes.String,
      activeTab: 'static-templates',
    }
  }

  public render() {
    const {onImport} = this.props
    const {activeTab, sortType, sortKey, sortDirection} = this.state

    const leftHeaderItems = (
      <>
        {this.filterComponent}
        <SelectGroup>
          <SelectGroup.Option
            name="template-type"
            id="static-templates"
            active={activeTab === 'static-templates'}
            value="static-templates"
            onClick={this.handleClickTab}
            titleText="Static Templates"
          >
            Static Templates
          </SelectGroup.Option>
          <SelectGroup.Option
            name="template-type"
            id="user-templates"
            active={activeTab === 'user-templates'}
            value="user-templates"
            onClick={this.handleClickTab}
            titleText="User Templates"
          >
            User Templates
          </SelectGroup.Option>
        </SelectGroup>
        <ResourceSortDropdown
          resourceType={ResourceType.Templates}
          sortType={sortType}
          sortKey={sortKey}
          sortDirection={sortDirection}
          onSelect={this.handleSort}
        />
      </>
    )

    return (
      <>
        <TabbedPageHeader
          childrenLeft={leftHeaderItems}
          childrenRight={
            <Button
              text="Import Template"
              icon={IconFont.Plus}
              color={ComponentColor.Primary}
              onClick={onImport}
            />
          }
        />
        {this.templatesList}
      </>
    )
  }

  private handleClickTab = val => {
    this.setState({activeTab: val})
  }

  private handleSort = (
    sortKey: TemplateSortKey,
    sortDirection: Sort,
    sortType: SortTypes
  ): void => {
    this.setState({sortKey, sortDirection, sortType})
  }

  private get templatesList(): JSX.Element {
    const {templates, onImport} = this.props
    const {searchTerm, sortKey, sortDirection, sortType, activeTab} = this.state

    if (activeTab === 'static-templates') {
      return (
        <FilterStaticTemplates
          searchTerm={searchTerm}
          searchKeys={['template.meta.name', 'labels[].name']}
          list={staticTemplates}
        >
          {ts => {
            return (
              <StaticTemplatesList
                searchTerm={searchTerm}
                templates={ts}
                onFilterChange={this.setSearchTerm}
                onImport={onImport}
                sortKey={sortKey}
                sortDirection={sortDirection}
                sortType={sortType}
              />
            )
          }}
        </FilterStaticTemplates>
      )
    }

    if (activeTab === 'user-templates') {
      return (
        <GetResources resources={[ResourceType.Labels]}>
          <FilterTemplateSummaries
            searchTerm={searchTerm}
            searchKeys={['meta.name', 'labels[].name']}
            list={templates}
          >
            {ts => {
              return (
                <TemplatesList
                  searchTerm={searchTerm}
                  templates={ts}
                  onFilterChange={this.setSearchTerm}
                  onImport={onImport}
                  sortKey={sortKey}
                  sortDirection={sortDirection}
                  sortType={sortType}
                />
              )
            }}
          </FilterTemplateSummaries>
        </GetResources>
      )
    }
  }

  private get filterComponent(): JSX.Element {
    const {searchTerm} = this.state

    return (
      <SearchWidget
        placeholderText="Filter templates..."
        onSearch={this.setSearchTerm}
        searchTerm={searchTerm}
      />
    )
  }

  private setSearchTerm = (searchTerm: string) => {
    this.setState({searchTerm})
  }
}
const mstp = (state: AppState) => ({
  templates: getAll(state, ResourceType.Templates),
})

const connector = connect(mstp)

export default connector(TemplatesPage)
