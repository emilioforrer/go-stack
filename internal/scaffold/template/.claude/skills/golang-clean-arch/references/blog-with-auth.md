# Blog Application with Authentication

This reference demonstrates the architecture defined by the parent
`golang-design-patterns` skill through a complete blog example.

It is illustrative rather than prescriptive.

Create only packages required by the application. Do not reproduce every
directory automatically.

## Business Capabilities

The example contains four isolated business features:

* `auth` owns accounts, credentials, access tokens, and sessions;
* `profile` owns public author information;
* `post` owns blog post creation and publication;
* `comment` owns comments and comment lifecycle.

It also contains two cross-feature workflows:

* `registerauthor` coordinates account and profile creation;
* `publishcomment` verifies that a post accepts comments and then creates the
  comment.

## Complete Project Structure

```text
blog/
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   └── migrate/
│       └── main.go
│
├── internal/
│   ├── appconfig/
│   │   ├── config.go
│   │   ├── load.go
│   │   └── validate.go
│   │
│   ├── features/
│   │   ├── auth/
│   │   │   ├── README.md
│   │   │   ├── package.go
│   │   │   ├── config/
│   │   │   │   └── config.go
│   │   │   ├── domain/
│   │   │   │   ├── account.go
│   │   │   │   ├── credential.go
│   │   │   │   ├── session.go
│   │   │   │   ├── role.go
│   │   │   │   ├── errors.go
│   │   │   │   └── events.go
│   │   │   ├── application/
│   │   │   │   ├── port/
│   │   │   │   │   ├── incoming/
│   │   │   │   │   │   ├── register_account.go
│   │   │   │   │   │   ├── authenticate.go
│   │   │   │   │   │   ├── refresh_session.go
│   │   │   │   │   │   ├── revoke_session.go
│   │   │   │   │   │   └── verify_access_token.go
│   │   │   │   │   └── outgoing/
│   │   │   │   │       ├── account_repository.go
│   │   │   │   │       ├── session_repository.go
│   │   │   │   │       ├── password_hasher.go
│   │   │   │   │       ├── token_issuer.go
│   │   │   │   │       └── event_publisher.go
│   │   │   │   ├── usecase/
│   │   │   │   │   ├── registeraccount/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── errors.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── authenticate/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── errors.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── refreshsession/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── revokesession/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   └── verifyaccesstoken/
│   │   │   │   │       ├── input.go
│   │   │   │   │       ├── output.go
│   │   │   │   │       ├── handler.go
│   │   │   │   │       ├── handler_test.go
│   │   │   │   │       └── package.go
│   │   │   │   └── service/
│   │   │   │       ├── service.go
│   │   │   │       ├── service_test.go
│   │   │   │       └── package.go
│   │   │   └── adapter/
│   │   │       ├── incoming/
│   │   │       │   └── httpapi/
│   │   │       │       ├── config/
│   │   │       │       │   └── config.go
│   │   │       │       ├── routes.go
│   │   │       │       ├── authenticate_request.go
│   │   │       │       ├── authenticate_response.go
│   │   │       │       ├── authenticate_handler.go
│   │   │       │       ├── refresh_request.go
│   │   │       │       ├── refresh_response.go
│   │   │       │       ├── refresh_handler.go
│   │   │       │       ├── revoke_handler.go
│   │   │       │       ├── error_mapper.go
│   │   │       │       └── package.go
│   │   │       └── outgoing/
│   │   │           ├── persistence/
│   │   │           │   └── postgres/
│   │   │           │       ├── config/
│   │   │           │       │   └── config.go
│   │   │           │       ├── migrations/
│   │   │           │       │   ├── 001_create_accounts.sql
│   │   │           │       │   └── 002_create_credentials.sql
│   │   │           │       ├── account_model.go
│   │   │           │       ├── account_mapper.go
│   │   │           │       ├── account_repository.go
│   │   │           │       ├── account_repository_test.go
│   │   │           │       └── package.go
│   │   │           ├── session/
│   │   │           │   └── redis/
│   │   │           │       ├── config/
│   │   │           │       │   └── config.go
│   │   │           │       ├── session_repository.go
│   │   │           │       ├── session_repository_test.go
│   │   │           │       └── package.go
│   │   │           ├── password/
│   │   │           │   └── argon2id/
│   │   │           │       ├── config/
│   │   │           │       │   └── config.go
│   │   │           │       ├── hasher.go
│   │   │           │       ├── hasher_test.go
│   │   │           │       └── package.go
│   │   │           ├── token/
│   │   │           │   └── jwt/
│   │   │           │       ├── config/
│   │   │           │       │   └── config.go
│   │   │           │       ├── issuer.go
│   │   │           │       ├── verifier.go
│   │   │           │       ├── token_test.go
│   │   │           │       └── package.go
│   │   │           └── messaging/
│   │   │               └── nats/
│   │   │                   ├── config/
│   │   │                   │   └── config.go
│   │   │                   ├── event_publisher.go
│   │   │                   └── package.go
│   │   │
│   │   ├── profile/
│   │   │   ├── README.md
│   │   │   ├── package.go
│   │   │   ├── config/
│   │   │   │   └── config.go
│   │   │   ├── domain/
│   │   │   │   ├── profile.go
│   │   │   │   ├── display_name.go
│   │   │   │   ├── biography.go
│   │   │   │   ├── avatar.go
│   │   │   │   ├── errors.go
│   │   │   │   └── events.go
│   │   │   ├── application/
│   │   │   │   ├── port/
│   │   │   │   │   ├── incoming/
│   │   │   │   │   │   ├── create_profile.go
│   │   │   │   │   │   ├── get_profile.go
│   │   │   │   │   │   └── update_profile.go
│   │   │   │   │   └── outgoing/
│   │   │   │   │       ├── profile_repository.go
│   │   │   │   │       └── avatar_storage.go
│   │   │   │   ├── usecase/
│   │   │   │   │   ├── createprofile/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── getprofile/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   └── updateprofile/
│   │   │   │   │       ├── input.go
│   │   │   │   │       ├── output.go
│   │   │   │   │       ├── handler.go
│   │   │   │   │       ├── handler_test.go
│   │   │   │   │       └── package.go
│   │   │   │   └── service/
│   │   │   │       ├── service.go
│   │   │   │       └── package.go
│   │   │   └── adapter/
│   │   │       ├── incoming/
│   │   │       │   └── httpapi/
│   │   │       │       ├── config/
│   │   │       │       │   └── config.go
│   │   │       │       ├── routes.go
│   │   │       │       ├── get_profile_handler.go
│   │   │       │       ├── update_profile_handler.go
│   │   │       │       ├── request.go
│   │   │       │       ├── response.go
│   │   │       │       ├── error_mapper.go
│   │   │       │       └── package.go
│   │   │       └── outgoing/
│   │   │           ├── persistence/
│   │   │           │   └── postgres/
│   │   │           │       ├── config/
│   │   │           │       │   └── config.go
│   │   │           │       ├── migrations/
│   │   │           │       │   └── 001_create_profiles.sql
│   │   │           │       ├── model.go
│   │   │           │       ├── mapper.go
│   │   │           │       ├── repository.go
│   │   │           │       └── package.go
│   │   │           └── avatar/
│   │   │               └── s3/
│   │   │                   ├── config/
│   │   │                   │   └── config.go
│   │   │                   ├── storage.go
│   │   │                   └── package.go
│   │   │
│   │   ├── post/
│   │   │   ├── README.md
│   │   │   ├── package.go
│   │   │   ├── config/
│   │   │   │   └── config.go
│   │   │   ├── domain/
│   │   │   │   ├── post.go
│   │   │   │   ├── title.go
│   │   │   │   ├── content.go
│   │   │   │   ├── slug.go
│   │   │   │   ├── status.go
│   │   │   │   ├── tag.go
│   │   │   │   ├── errors.go
│   │   │   │   └── events.go
│   │   │   ├── application/
│   │   │   │   ├── port/
│   │   │   │   │   ├── incoming/
│   │   │   │   │   │   ├── create_draft.go
│   │   │   │   │   │   ├── update_post.go
│   │   │   │   │   │   ├── publish_post.go
│   │   │   │   │   │   ├── get_post.go
│   │   │   │   │   │   ├── list_posts.go
│   │   │   │   │   │   ├── delete_post.go
│   │   │   │   │   │   └── check_commenting.go
│   │   │   │   │   └── outgoing/
│   │   │   │   │       ├── post_repository.go
│   │   │   │   │       └── event_publisher.go
│   │   │   │   ├── usecase/
│   │   │   │   │   ├── createdraft/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── updatepost/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── publishpost/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── getpost/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── listposts/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   ├── deletepost/
│   │   │   │   │   │   ├── input.go
│   │   │   │   │   │   ├── output.go
│   │   │   │   │   │   ├── handler.go
│   │   │   │   │   │   ├── handler_test.go
│   │   │   │   │   │   └── package.go
│   │   │   │   │   └── checkcommenting/
│   │   │   │   │       ├── input.go
│   │   │   │   │       ├── output.go
│   │   │   │   │       ├── handler.go
│   │   │   │   │       ├── handler_test.go
│   │   │   │   │       └── package.go
│   │   │   │   └── service/
│   │   │   │       ├── service.go
│   │   │   │       └── package.go
│   │   │   └── adapter/
│   │   │       ├── incoming/
│   │   │       │   └── httpapi/
│   │   │       │       ├── config/
│   │   │       │       │   └── config.go
│   │   │       │       ├── routes.go
│   │   │       │       ├── create_draft_handler.go
│   │   │       │       ├── update_post_handler.go
│   │   │       │       ├── publish_post_handler.go
│   │   │       │       ├── get_post_handler.go
│   │   │       │       ├── list_posts_handler.go
│   │   │       │       ├── delete_post_handler.go
│   │   │       │       ├── request.go
│   │   │       │       ├── response.go
│   │   │       │       ├── error_mapper.go
│   │   │       │       └── package.go
│   │   │       └── outgoing/
│   │   │           ├── persistence/
│   │   │           │   └── postgres/
│   │   │           │       ├── config/
│   │   │           │       │   └── config.go
│   │   │           │       ├── migrations/
│   │   │           │       │   ├── 001_create_posts.sql
│   │   │           │       │   └── 002_create_post_tags.sql
│   │   │           │       ├── post_model.go
│   │   │           │       ├── tag_model.go
│   │   │           │       ├── mapper.go
│   │   │           │       ├── repository.go
│   │   │           │       └── package.go
│   │   │           └── messaging/
│   │   │               └── nats/
│   │   │                   ├── config/
│   │   │                   │   └── config.go
│   │   │                   ├── event_publisher.go
│   │   │                   └── package.go
│   │   │
│   │   └── comment/
│   │       ├── README.md
│   │       ├── package.go
│   │       ├── config/
│   │       │   └── config.go
│   │       ├── domain/
│   │       │   ├── comment.go
│   │       │   ├── content.go
│   │       │   ├── status.go
│   │       │   ├── errors.go
│   │       │   └── events.go
│   │       ├── application/
│   │       │   ├── port/
│   │       │   │   ├── incoming/
│   │       │   │   │   ├── create_comment.go
│   │       │   │   │   ├── list_comments.go
│   │       │   │   │   └── delete_comment.go
│   │       │   │   └── outgoing/
│   │       │   │       ├── comment_repository.go
│   │       │   │       └── event_publisher.go
│   │       │   ├── usecase/
│   │       │   │   ├── createcomment/
│   │       │   │   │   ├── input.go
│   │       │   │   │   ├── output.go
│   │       │   │   │   ├── handler.go
│   │       │   │   │   ├── handler_test.go
│   │       │   │   │   └── package.go
│   │       │   │   ├── listcomments/
│   │       │   │   │   ├── input.go
│   │       │   │   │   ├── output.go
│   │       │   │   │   ├── handler.go
│   │       │   │   │   ├── handler_test.go
│   │       │   │   │   └── package.go
│   │       │   │   └── deletecomment/
│   │       │   │       ├── input.go
│   │       │   │       ├── output.go
│   │       │   │       ├── handler.go
│   │       │   │       ├── handler_test.go
│   │       │   │       └── package.go
│   │       │   └── service/
│   │       │       ├── service.go
│   │       │       └── package.go
│   │       └── adapter/
│   │           ├── incoming/
│   │           │   └── httpapi/
│   │           │       ├── config/
│   │           │       │   └── config.go
│   │           │       ├── routes.go
│   │           │       ├── list_comments_handler.go
│   │           │       ├── delete_comment_handler.go
│   │           │       ├── response.go
│   │           │       ├── error_mapper.go
│   │           │       └── package.go
│   │           └── outgoing/
│   │               ├── persistence/
│   │               │   └── postgres/
│   │               │       ├── config/
│   │               │       │   └── config.go
│   │               │       ├── migrations/
│   │               │       │   └── 001_create_comments.sql
│   │               │       ├── model.go
│   │               │       ├── mapper.go
│   │               │       ├── repository.go
│   │               │       └── package.go
│   │               └── messaging/
│   │                   └── nats/
│   │                       ├── config/
│   │                       │   └── config.go
│   │                       ├── event_publisher.go
│   │                       └── package.go
│   │
│   ├── workflows/
│   │   ├── registerauthor/
│   │   │   ├── README.md
│   │   │   ├── package.go
│   │   │   ├── config/
│   │   │   │   └── config.go
│   │   │   ├── input.go
│   │   │   ├── output.go
│   │   │   ├── errors.go
│   │   │   ├── workflow.go
│   │   │   ├── workflow_test.go
│   │   │   └── adapter/
│   │   │       └── incoming/
│   │   │           └── httpapi/
│   │   │               ├── config/
│   │   │               │   └── config.go
│   │   │               ├── routes.go
│   │   │               ├── request.go
│   │   │               ├── response.go
│   │   │               ├── handler.go
│   │   │               ├── error_mapper.go
│   │   │               └── package.go
│   │   └── publishcomment/
│   │       ├── README.md
│   │       ├── package.go
│   │       ├── config/
│   │       │   └── config.go
│   │       ├── input.go
│   │       ├── output.go
│   │       ├── errors.go
│   │       ├── workflow.go
│   │       ├── workflow_test.go
│   │       └── adapter/
│   │           └── incoming/
│   │               └── httpapi/
│   │                   ├── config/
│   │                   │   └── config.go
│   │                   ├── routes.go
│   │                   ├── request.go
│   │                   ├── response.go
│   │                   ├── handler.go
│   │                   ├── error_mapper.go
│   │                   └── package.go
│   │
│   └── shared/
│       ├── domain/
│       │   └── identity/
│       │       ├── userid.go
│       │       ├── userid_test.go
│       │       └── errors.go
│       ├── application/
│       │   ├── clock/
│       │   │   ├── clock.go
│       │   │   ├── system.go
│       │   │   └── package.go
│       │   ├── idgenerator/
│       │   │   ├── generator.go
│       │   │   ├── uuid.go
│       │   │   └── package.go
│       │   └── principal/
│       │       ├── principal.go
│       │       ├── authenticator.go
│       │       └── context.go
│       └── infrastructure/
│           ├── database/
│           │   └── postgres/
│           │       ├── config/
│           │       │   └── config.go
│           │       ├── db.go
│           │       ├── connection.go
│           │       ├── migrations.go
│           │       └── package.go
│           ├── cache/
│           │   └── redis/
│           │       ├── config/
│           │       │   └── config.go
│           │       ├── client.go
│           │       └── package.go
│           ├── messaging/
│           │   └── nats/
│           │       ├── config/
│           │       │   └── config.go
│           │       ├── connection.go
│           │       └── package.go
│           ├── observability/
│           │   ├── config/
│           │   │   └── config.go
│           │   ├── logging.go
│           │   ├── telemetry.go
│           │   └── package.go
│           └── httpserver/
│               ├── config/
│               │   └── config.go
│               ├── server.go
│               ├── router.go
│               ├── package.go
│               └── middleware/
│                   ├── authentication.go
│                   ├── authorization.go
│                   ├── recovery.go
│                   ├── request_id.go
│                   └── logging.go
│
├── test/
│   ├── integration/
│   │   ├── auth_test.go
│   │   ├── profile_test.go
│   │   ├── post_test.go
│   │   ├── comment_test.go
│   │   ├── register_author_test.go
│   │   └── publish_comment_test.go
│   └── endtoend/
│       ├── authentication_test.go
│       ├── author_registration_test.go
│       ├── post_publishing_test.go
│       └── commenting_test.go
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Feature Responsibilities

### Auth

The `auth` feature owns:

* accounts;
* credentials;
* password verification;
* roles used for authentication and authorization;
* access-token creation and verification;
* refresh sessions;
* session revocation.

It does not own:

* public display names;
* biographies;
* avatars;
* posts;
* comments.

Those concepts belong to other business features.

### Profile

The `profile` feature owns:

* public author profiles;
* display names;
* biographies;
* avatars;
* profile updates.

It identifies the profile using a shared user identifier.

It does not import the `auth` feature.

### Post

The `post` feature owns:

* drafts;
* post titles;
* content;
* slugs;
* tags;
* publication state;
* deletion;
* whether a post currently accepts comments.

It receives the authenticated author ID as application input.

It does not import `auth` or `profile`.

### Comment

The `comment` feature owns:

* comment content;
* comment authorship;
* moderation status;
* comment deletion;
* listing comments for a post.

It stores the target post ID as data but does not import the `post` feature.

## Author Registration Workflow

Creating an author requires two independent business capabilities:

1. create an authentication account;
2. create a public author profile.

The workflow owns that coordination:

```text
HTTP request
    │
    ▼
registerauthor workflow
    ├──► auth.RegisterAccount
    └──► profile.CreateProfile
```

The workflow imports only incoming ports:

```text
workflows/registerauthor
    ├──► features/auth/application/port/incoming
    └──► features/profile/application/port/incoming
```

It does not import:

* the auth domain;
* the profile domain;
* either feature's outgoing ports;
* either feature's repositories;
* either feature's adapters;
* concrete use-case handlers.

If profile creation fails after account creation, the workflow decides the
appropriate cross-feature recovery behavior. Depending on the business
requirement, that may be:

* invoking an auth capability that disables or removes the account;
* storing workflow state for retry;
* publishing a recovery event;
* leaving the account incomplete for later reconciliation.

The workflow must not manipulate the auth repository directly.

## Comment Publication Workflow

Publishing a comment involves two capabilities:

1. verify that the target post exists and accepts comments;
2. create the comment.

```text
HTTP request
    │
    ▼
publishcomment workflow
    ├──► post.CheckCommenting
    └──► comment.CreateComment
```

The post feature exposes the business decision through an incoming port.

The workflow must not load the post through the post repository or interpret
the post domain directly.

## Authentication Across Features

Authentication is implemented by the `auth` feature, but other features should
not import that feature directly.

A shared application contract represents the authenticated caller:

```text
shared/application/principal/
├── principal.go
├── authenticator.go
└── context.go
```

Conceptually:

```go
package principal

import "context"

type Principal struct {
	UserID string
	Roles  []string
}

type Authenticator interface {
	Authenticate(
		ctx context.Context,
		accessToken string,
	) (Principal, error)
}
```

The auth feature implements `principal.Authenticator`.

The HTTP middleware depends on the shared contract:

```text
HTTP middleware
    │
    ├──► principal.Authenticator
    │          ▲
    │          │
    │     auth implementation
    │
    ▼
principal placed in request context
    │
    ▼
feature or workflow HTTP adapter
```

Post, profile, comment, and workflow adapters consume the principal from the
request context without importing the auth feature.

The shared principal contract must remain small and independent from concrete
auth domain types.

## Shared Identity

Features may need to refer to the same authenticated user without depending on
the auth feature.

A shared semantic identity may be placed under:

```text
shared/domain/identity
```

For example:

```go
package identity

type UserID string
```

This is appropriate only when `UserID` has the same meaning across auth,
profile, post, and comment.

Feature-specific identities remain inside their owning features:

```text
auth.AccountID
profile.ProfileID
post.PostID
comment.CommentID
```

## Dependency Direction

```text
cmd/api
  ├──► auth
  ├──► profile
  ├──► post
  ├──► comment
  ├──► registerauthor workflow
  ├──► publishcomment workflow
  └──► shared infrastructure

registerauthor workflow
  ├──► auth incoming ports
  └──► profile incoming ports

publishcomment workflow
  ├──► post incoming ports
  └──► comment incoming ports

features
  ─X► other features

features
  ─X► workflows

workflows
  ─X► feature domains
  ─X► feature outgoing ports
  ─X► feature repositories
  ─X► feature adapters

shared
  ─X► features
  ─X► workflows
```

## Example Feature Package Registration

The auth feature config aggregates its child configurations:

```go
package config

import (
	httpconfig "example.com/blog/internal/features/auth/adapter/incoming/httpapi/config"
	postgresconfig "example.com/blog/internal/features/auth/adapter/outgoing/persistence/postgres/config"
	redisconfig "example.com/blog/internal/features/auth/adapter/outgoing/session/redis/config"
	argonconfig "example.com/blog/internal/features/auth/adapter/outgoing/password/argon2id/config"
	jwtconfig "example.com/blog/internal/features/auth/adapter/outgoing/token/jwt/config"
)

type Config struct {
	HTTPAPI  httpconfig.Config
	Postgres postgresconfig.Config
	Redis    redisconfig.Config
	Argon2ID argonconfig.Config
	JWT      jwtconfig.Config
}
```

The feature package composes child registrations:

```go
package auth

import (
	"github.com/samber/do/v2"

	"example.com/blog/internal/features/auth/adapter/incoming/httpapi"
	authpostgres "example.com/blog/internal/features/auth/adapter/outgoing/persistence/postgres"
	"example.com/blog/internal/features/auth/adapter/outgoing/password/argon2id"
	sessionredis "example.com/blog/internal/features/auth/adapter/outgoing/session/redis"
	"example.com/blog/internal/features/auth/adapter/outgoing/token/jwt"
	"example.com/blog/internal/features/auth/application/service"
	"example.com/blog/internal/features/auth/application/usecase/authenticate"
	"example.com/blog/internal/features/auth/application/usecase/refreshsession"
	"example.com/blog/internal/features/auth/application/usecase/registeraccount"
	"example.com/blog/internal/features/auth/application/usecase/revokesession"
	"example.com/blog/internal/features/auth/application/usecase/verifyaccesstoken"
	authconfig "example.com/blog/internal/features/auth/config"
)

func Package(config authconfig.Config) func(do.Injector) {
	return do.Package(
		authpostgres.Package(config.Postgres),
		sessionredis.Package(config.Redis),
		argon2id.Package(config.Argon2ID),
		jwt.Package(config.JWT),

		registeraccount.Package,
		authenticate.Package,
		refreshsession.Package,
		revokesession.Package,
		verifyaccesstoken.Package,
		service.Package,

		httpapi.Package(config.HTTPAPI),
	)
}
```

Child packages do not import the root `auth` package.

## Example Workflow Registration

The registration workflow receives feature capabilities through incoming ports:

```go
package registerauthor

import (
	"github.com/samber/do/v2"

	authincoming "example.com/blog/internal/features/auth/application/port/incoming"
	profileincoming "example.com/blog/internal/features/profile/application/port/incoming"
	workflowconfig "example.com/blog/internal/workflows/registerauthor/config"
)

func provideWorkflow(i do.Injector) (*Workflow, error) {
	return NewWorkflow(
		do.MustInvoke[authincoming.RegisterAccount](i),
		do.MustInvoke[profileincoming.CreateProfile](i),
		do.MustInvoke[workflowconfig.Config](i),
	), nil
}

func Package(config workflowconfig.Config) func(do.Injector) {
	return do.Package(
		do.Eager(config),
		do.Lazy(provideWorkflow),
	)
}
```

The workflow constructor remains independent from the dependency container:

```go
func NewWorkflow(
	registerAccount authincoming.RegisterAccount,
	createProfile profileincoming.CreateProfile,
	config workflowconfig.Config,
) *Workflow {
	return &Workflow{
		registerAccount: registerAccount,
		createProfile:   createProfile,
		config:          config,
	}
}
```

## Application Configuration

The application config aggregates feature, workflow, and shared infrastructure
configuration:

```go
package appconfig

import (
	authconfig "example.com/blog/internal/features/auth/config"
	commentconfig "example.com/blog/internal/features/comment/config"
	postconfig "example.com/blog/internal/features/post/config"
	profileconfig "example.com/blog/internal/features/profile/config"
	postgresconfig "example.com/blog/internal/shared/infrastructure/database/postgres/config"
	httpserverconfig "example.com/blog/internal/shared/infrastructure/httpserver/config"
	observabilityconfig "example.com/blog/internal/shared/infrastructure/observability/config"
	publishcommentconfig "example.com/blog/internal/workflows/publishcomment/config"
	registerauthorconfig "example.com/blog/internal/workflows/registerauthor/config"
)

type Config struct {
	Auth           authconfig.Config
	Profile        profileconfig.Config
	Post           postconfig.Config
	Comment        commentconfig.Config
	RegisterAuthor registerauthorconfig.Config
	PublishComment publishcommentconfig.Config
	Postgres       postgresconfig.Config
	HTTPServer     httpserverconfig.Config
	Observability  observabilityconfig.Config
}
```

Application config imports package-owned config packages, not feature or workflow
implementation packages.

## Composition Root

The API executable selects the feature and workflow packages it needs:

```go
injector := do.New(
	observability.Package(config.Observability),
	postgres.Package(config.Postgres),
	httpserver.Package(config.HTTPServer),

	auth.Package(config.Auth),
	profile.Package(config.Profile),
	post.Package(config.Post),
	comment.Package(config.Comment),

	registerauthor.Package(config.RegisterAuthor),
	publishcomment.Package(config.PublishComment),
)
```

The composition root selects packages and controls lifecycle. It does not
implement registration, publication, authentication, or commenting business
rules.

## Important Boundaries Demonstrated

This example demonstrates that:

* auth and profile are separate capabilities;
* creating an author is a cross-feature workflow;
* features do not import auth merely because they require an authenticated user;
* authentication is exposed through a narrow shared principal contract;
* features may share a stable user identity without sharing their complete
  domain models;
* workflows consume incoming ports rather than repositories;
* feature configuration aggregates leaf config packages;
* dependency registration is exported through `package.go`;
* the complete folder tree is illustrative and should be reduced when the
  application requires fewer components.
