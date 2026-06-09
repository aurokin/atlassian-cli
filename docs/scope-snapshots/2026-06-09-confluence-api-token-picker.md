# Confluence API-token picker snapshot: 2026-06-09

Cached reflection of the Confluence scopes visible in the Atlassian API-token
picker on 2026-06-09. This is reference material only; use
[../token-scopes.md](../token-scopes.md#confluence) for the curated starter set.

| Type | Scope | Picker description |
|---|---|---|
| granular | `delete:attachment:confluence` | Delete attachments. |
| granular | `delete:blogpost:confluence` | Delete blogposts. |
| granular | `delete:comment:confluence` | Delete comments on content. |
| granular | `delete:content:confluence` | Delete content. |
| granular | `delete:custom-content:confluence` | Delete custom content. |
| granular | `delete:database:confluence` | Delete databases. |
| granular | `delete:embed:confluence` | Delete Smart Links. |
| granular | `delete:folder:confluence` | Delete folders. |
| granular | `delete:page:confluence` | Delete pages. |
| granular | `delete:space:confluence` | Delete spaces. |
| granular | `delete:whiteboard:confluence` | Delete whiteboards. |
| classic | `manage:confluence-configuration` | Manage global settings. |
|  | `manage:org` |  |
| classic | `read:account` | Required to view users profiles. |
| granular | `read:analytics.content:confluence` | View analytics for content. Note that this does not provide access to the content itself. |
| granular | `read:app-data:confluence` | Read connect app properties data. |
| granular | `read:attachment:confluence` | View and download attachments of a page or blogpost that you have access to. |
| granular | `read:audit-log:confluence` | View and export audit records for Confluence events. |
| granular | `read:blogpost:confluence` | View blogpost content. |
| granular | `read:comment:confluence` | View comments on pages or blogposts. |
| granular | `read:configuration:confluence` | View Confluence settings, themes and system information. |
| classic | `read:confluence-content.all` | Read all content, including content body (expansions permitted). Note, APIs using this scope may also return data allowed by read:confluence-space.summary. However, this scope is not a substitute for read:confluence-space.summary. |
| classic | `read:confluence-content.permission` | View content permission in Confluence. |
| classic | `read:confluence-content.summary` | Read a summary of the content, which is the content without expansions. Note, APIs using this scope may also return data allowed by read:confluence-space.summary. However, this scope is not a substitute for read:confluence-space.summary. |
| classic | `read:confluence-groups` | Permits retrieval of user groups. |
| classic | `read:confluence-props` | Read content properties. |
| classic | `read:confluence-space.summary` | Read a summary of space information without expansions. |
| classic | `read:confluence-user` | View user information in Confluence that you have access to, including usernames, email addresses and profile pictures. |
| granular | `read:content-details:confluence` | View details regarding content and its associated properties. |
| granular | `read:content:confluence` | View content, including pages, blogposts, custom content, attachments, comments, and content templates. |
| granular | `read:content.metadata:confluence` | View information about the content. Note that this does not provide access to the content itself. |
| granular | `read:content.permission:confluence` | Check if a user or a group can perform an operation to the specified content. |
| granular | `read:content.property:confluence` | View properties associated with a content. |
| granular | `read:content.restriction:confluence` | View the restrictions on the content. |
| granular | `read:custom-content:confluence` | View custom content. |
| granular | `read:database:confluence` | View database data, such as its content id and title. |
| granular | `read:email-address:confluence` | View email addresses of all users regardless of the user's profile visibility settings. |
| granular | `read:embed:confluence` | View Smart Link data, such as its content id and title. |
| granular | `read:folder:confluence` | View folder data, such as its content id and title. |
| granular | `read:group:confluence` | View details about groups including its members. |
| granular | `read:hierarchical-content:confluence` | View children or descendants for hierarchical content, including pages, whiteboards, databases, folders, and smart links. |
| granular | `read:inlinetask:confluence` | Search and view inline tasks. |
| granular | `read:label:confluence` | View labels associated with the content or space. |
| classic | `read:me` | View the profile details for the currently logged-in user. |
| granular | `read:page:confluence` | View page content. |
| granular | `read:permission:confluence` | View content restrictions and space permissions. Note that this is only used for V2 APIs. |
| granular | `read:relation:confluence` | View relationships between two entities. |
| granular | `read:space-details:confluence` | View details regarding spaces and their associated properties. |
| granular | `read:space:confluence` | View space details. |
| granular | `read:space.permission:confluence` | View space permissions. |
| granular | `read:space.property:confluence` | View properties associated with the space. |
| granular | `read:space.setting:confluence` | View space settings and themes. |
| granular | `read:task:confluence` | View Confluence tasks. Note that this is only used for V2 APIs. |
| granular | `read:template:confluence` | View content templates. |
| granular | `read:user:confluence` | View user details. |
| granular | `read:user.property:confluence` | View properties associated with the user. |
| granular | `read:watcher:confluence` | View the watchers associated with the contents, spaces or labels. |
| granular | `read:whiteboard:confluence` | View whiteboard data, such as its content id and title. |
| classic | `readonly:content.attachment:confluence` | Download attachments of a Confluence page or blogpost that you have access to. |
| classic | `search:confluence` | Search Confluence. Note, APIs using this scope may also return data allowed by read:confluence-space.summary and read:confluence-content.summary. However, this scope is not a substitute for read:confluence-space.summary or read:confluence-content.summary. |
| granular | `write:app-data:confluence` | Create, modify and delete app properties data. |
| granular | `write:attachment:confluence` | Create and update attachments. |
| granular | `write:audit-log:confluence` | Create records in the audit log. |
| granular | `write:blogpost:confluence` | Create and update blogposts. |
| granular | `write:comment:confluence` | Create and update comments on content. |
| granular | `write:configuration:confluence` | Update Confluence settings, including global look and feel. |
| classic | `write:confluence-content` | Permits the creation of pages, blogs, comments and questions. |
| classic | `write:confluence-file` | Upload attachments. |
| classic | `write:confluence-groups` | Permits creation, removal and update of user groups. |
| classic | `write:confluence-props` | Write content properties. |
| classic | `write:confluence-space` | Create, update and delete space information. |
| granular | `write:content:confluence` | Create and update content and its associated properties. |
| granular | `write:content.property:confluence` | Create, update and delete properties associated with a content. |
| granular | `write:content.restriction:confluence` | Update the restrictions on the content. |
| granular | `write:custom-content:confluence` | Create and update custom content. |
| granular | `write:database:confluence` | Create and update databases. |
| granular | `write:embed:confluence` | Create and update Smart Links. |
| granular | `write:folder:confluence` | Create and update folders. |
| granular | `write:group:confluence` | Create, update, and delete groups. |
| granular | `write:inlinetask:confluence` | Mark inline tasks as complete or incomplete. |
| granular | `write:label:confluence` | Add and remove labels associated with the content or space. |
| granular | `write:page:confluence` | Create and update pages. |
| granular | `write:relation:confluence` | Create and update relationships between two entities. |
| granular | `write:space:confluence` | Create and update spaces. |
| granular | `write:space.permission:confluence` | Update space permissions. |
| granular | `write:space.property:confluence` | Create, update and delete properties associated with the space. |
| granular | `write:space.setting:confluence` | Update space settings and themes. |
| granular | `write:task:confluence` | Update Confluence tasks. Note that this is only used for V2 APIs. |
| granular | `write:template:confluence` | Create, update and delete content templates. |
| granular | `write:user.property:confluence` | Create, update and delete properties associated with the user. |
| granular | `write:watcher:confluence` | Add and remove the watchers associated with content, spaces, or labels. |
| granular | `write:whiteboard:confluence` | Create and update whiteboards. |
