import {
  initialState,
  notificationsReducer,
} from 'src/shared/reducers/notifications'

import {notify, dismissNotification} from 'src/shared/actions/notifications'

import {FIVE_SECONDS} from 'src/shared/constants/index'

import {IconFont} from '@influxdata/clockface'
import {NotificationStyle} from 'src/types/notifications'

const notificationID = '000'

const exampleNotification = {
  id: notificationID,
  style: NotificationStyle.Success,
  message: 'Hell yeah you are a real notification!',
  duration: FIVE_SECONDS,
  icon: IconFont.Zap,
}

const exampleNotifications = [exampleNotification]

describe('Shared.Reducers.notifications', () => {
  it('should publish a notification', () => {
    const [actual] = notificationsReducer(
      initialState,
      notify(exampleNotification)
    )

    const [expected] = [exampleNotification, ...initialState]

    expect(actual.style).toEqual(expected.style)
    expect(actual.icon).toEqual(expected.icon)
    expect(actual.message).toEqual(expected.message)
    expect(actual.duration).toEqual(expected.duration)
  })

  describe('adding more than one notification', () => {
    it('should put the new notification at the beginning of the list', () => {
      const newNotification = {
        style: NotificationStyle.Error,
        message: 'new notification',
        duration: FIVE_SECONDS,
        icon: IconFont.Zap,
      }

      const actual = notificationsReducer(
        exampleNotifications,
        notify(newNotification)
      )

      expect(actual.length).toBe(2)
      expect(actual[0].message).toEqual(newNotification.message)
    })
  })

  it('should dismiss a notification', () => {
    const actual = notificationsReducer(
      exampleNotifications,
      dismissNotification(notificationID)
    )

    expect(actual.length).toBe(0)
  })
})
