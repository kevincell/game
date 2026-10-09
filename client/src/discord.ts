import * as React from 'react'

import { MessageType } from './protocol'
import type { ClientAuthMessage, ServerAuthMessage } from './protocol'

enum AuthStatus {
  Unauthenticated,
  Authenticated,
  Failed,
  AvatarMounted,
}

export type DiscordUser = {
  Id: string
  Username: string
  Discriminator: string
  Avatar: string
}

export type UnauthenticatedState = {
  status: AuthStatus.Unauthenticated
}

export type AuthenticatedState = {
  status: AuthStatus.Authenticated
  user: DiscordUser
  key: string
}

export type FailedState = {
  status: AuthStatus.Failed
}

export type AvatarMountedState = {
  status: AuthStatus.AvatarMounted
  user: DiscordUser
  key: string
  avatarPath: string
}

export type AuthState =
  | UnauthenticatedState
  | AuthenticatedState
  | FailedState
  | AvatarMountedState

export const DISCORD_CODE = 'discord'

export function renderDiscordHeader(state: AuthState): string {
  return ''
}

export function renderDiscordButton(state: AuthState): string {
  if (
    state.status === AuthStatus.Authenticated ||
    state.status === AuthStatus.AvatarMounted
  ) {
    return `
          guibutton "discord.."        "showgui discord"
      `
  }

  return ``
}

export default function useAuth(
  sendMessage: (message: ClientAuthMessage) => void
): {
  state: AuthState
  initialize: (code: Maybe<string>) => void
  receiveMessage: (message: ServerAuthMessage) => void
} {
  const [state, setState] = React.useState<AuthState>({
    status: AuthStatus.Unauthenticated,
  })

  const initialize = React.useCallback(
    (urlCode: Maybe<string>) => {
      sendMessage({
        Op: MessageType.DiscordCode,
        Code: '',
      })
    },
    [sendMessage]
  )

  React.useEffect(() => {
    Module.discord = {
      login: () => {
      },
      copyKey: () => {
      },
      regenKey: () => {},
      logout: () => {
        localStorage.removeItem(DISCORD_CODE)
        setState({
          status: AuthStatus.Unauthenticated,
        })
      },
    }
  }, [state])

  const receiveMessage = React.useCallback((message: ServerAuthMessage) => {
    if (message.Op === MessageType.AuthSucceeded) {
      localStorage.setItem(DISCORD_CODE, message.Code)
      const { User: user, PrivateKey: key } = message
      setState({
        status: AuthStatus.Authenticated,
        user,
        key,
      })

      return
    }

    if (message.Op === MessageType.AuthFailed) {
      setState({
        status: AuthStatus.Failed,
      })

      setTimeout(() => {
        setState({
          status: AuthStatus.Unauthenticated,
        })
      }, 4000)
      return
    }
  }, [])

  const menu = React.useMemo<string>(() => {
    return ''
  }, [state])

  return {
    state,
    initialize,
    receiveMessage,
  }
}
