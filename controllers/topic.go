// Copyright 2021 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"encoding/json"
	"strconv"

	"github.com/casbin/casbin-forum/object"
	"github.com/casbin/casbin-forum/util"
)

type NewTopicForm struct {
	Title      string `json:"title"`
	Body       string `json:"body"`
	NodeId     string `json:"nodeId"`
	EditorType string `json:"editorType"`
}

func (c *APIController) GetTopics() {
	limitStr := c.Input().Get("limit")
	pageStr := c.Input().Get("page")
	defaultLimit := object.DefaultHomePageNum

	var limit, offset int
	if len(limitStr) != 0 {
		limit = util.ParseInt(limitStr)
	} else {
		limit = defaultLimit
	}
	if len(pageStr) != 0 {
		page := util.ParseInt(pageStr)
		offset = page*limit - limit
	}

	c.Data["json"] = object.GetTopics(limit, offset)
	c.ServeJSON()
}

func (c *APIController) GetTopicsAdmin() {
	limitStr := c.Input().Get("limit")
	pageStr := c.Input().Get("page")

	usernameSearchKw := c.Input().Get("un") // search: username(author)
	titleSearchKw := c.Input().Get("ti")    // search: title
	contentSearchKw := c.Input().Get("cn")  // search: content

	showDeletedTopics := c.Input().Get("sdt") // sort: show deleted topics

	createdTimeSort := c.Input().Get("cs") // sort: created time
	lastReplySort := c.Input().Get("lrs")  // sort: last reply time
	usernameSort := c.Input().Get("us")    // sort: username
	replyCountSort := c.Input().Get("rcs") // sort: reply count
	hotSort := c.Input().Get("hs")         // sort: hot
	favCountSort := c.Input().Get("fcs")   // sort: favorite count

	defaultLimit := object.DefaultHomePageNum

	var limit, offset int
	if len(limitStr) != 0 {
		limit = util.ParseInt(limitStr)
	} else {
		limit = defaultLimit
	}
	if len(pageStr) != 0 {
		page := util.ParseInt(pageStr)
		offset = page*limit - limit
	}

	res, num := object.GetTopicsAdmin(usernameSearchKw, titleSearchKw, contentSearchKw, showDeletedTopics, createdTimeSort, lastReplySort, usernameSort, replyCountSort, hotSort, favCountSort, limit, offset)

	c.Data["json"] = Response{Status: "ok", Msg: "success", Data: res, Data2: num}
	c.ServeJSON()
}

func (c *APIController) GetTopic() {
	memberId := c.GetSessionUser()
	idStr := c.Input().Get("id")

	id := util.ParseInt(idStr)

	topic := object.GetTopicWithAvatar(id, memberId)
	if topic == nil || topic.Deleted {
		c.Data["json"] = nil
		c.ServeJSON()
		return
	}

	if memberId != "" {
		topic.NodeModerator = object.CheckNodeModerator(memberId, topic.NodeId)
	}

	c.Data["json"] = topic
	c.ServeJSON()
}

func (c *APIController) GetTopicAdmin() {
	idStr := c.Input().Get("id")

	id := util.ParseInt(idStr)

	c.Data["json"] = object.GetTopicAdmin(id)
	c.ServeJSON()
}

func (c *APIController) UpdateTopic() {
	idStr := c.Input().Get("id")

	var topic object.Topic
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &topic)
	if err != nil {
		panic(err)
	}

	id := util.ParseInt(idStr)
	c.Data["json"] = object.UpdateTopic(id, &topic)
	c.ServeJSON()
}

func (c *APIController) AddTopic() {
	if c.RequireLogin() {
		return
	}

	memberId := c.GetSessionUser()
	// check account status
	if object.IsMuted(memberId) || object.IsForbidden(memberId) {
		c.mutedAccountResp(memberId)
		return
	}

	var form NewTopicForm
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &form)
	if err != nil {
		panic(err)
	}
	title, body, nodeId, editorType := form.Title, form.Body, form.NodeId, form.EditorType

	if object.ContainsSensitiveWord(title) {
		resp := Response{Status: "fail", Msg: "Topic title contains sensitive word."}
		c.Data["json"] = resp
		c.ServeJSON()
		return
	}

	if object.ContainsSensitiveWord(body) {
		resp := Response{Status: "fail", Msg: "Topic body contains sensitive word."}
		c.Data["json"] = resp
		c.ServeJSON()
		return
	}

	topic := object.Topic{
		//Id:            util.IntToString(object.GetTopicId()),
		Author:        memberId,
		NodeId:        nodeId,
		NodeName:      "",
		Title:         title,
		CreatedTime:   util.GetCurrentTime(),
		Tags:          nil,
		LastReplyUser: "",
		LastReplyTime: util.GetCurrentTime(),
		UpCount:       0,
		HitCount:      0,
		FavoriteCount: 0,
		Content:       body,
		Deleted:       false,
		EditorType:    editorType,
	}

	balance := object.GetMemberBalance(memberId)
	if balance < object.CreateTopicCost {
		resp := Response{Status: "fail", Msg: "You don't have enough balance."}
		c.Data["json"] = resp
		c.ServeJSON()
		return
	}
	//payRes := object.CreateTopicConsumption(c.GetSessionUser(), topic.Id)

	//object.AddTopicNotification(topic.Id, c.GetSessionUser(), body)

	err = json.Unmarshal(c.Ctx.Input.RequestBody, &topic)
	if err != nil {
		panic(err)
	}

	var resp Response
	res, id := object.AddTopic(&topic)
	if res {
		object.CreateTopicConsumption(topic.Author, id)
		object.AddTopicNotification(id, topic.Author, topic.Content)
		resp = Response{Status: "ok", Msg: "success", Data: topic.Id}
	} else {
		resp = Response{Status: "error", Msg: "fail"}
	}

	c.Data["json"] = resp
	c.ServeJSON()
}

func (c *APIController) DeleteTopic() {
	idStr := c.Input().Get("id")
	memberId := c.GetSessionUser()

	id := util.ParseInt(idStr)
	nodeId := object.GetTopicNodeId(id)
	if !object.CheckModIdentity(memberId) && !object.CheckNodeModerator(memberId, nodeId) {
		resp := Response{Status: "fail", Msg: "Unauthorized."}
		c.Data["json"] = resp
		c.ServeJSON()
		return
	}

	c.Data["json"] = object.DeleteTopic(id)
	c.ServeJSON()
}

func (c *APIController) GetTopicsNum() {
	c.Data["json"] = object.GetTopicNum()
	c.ServeJSON()
}

func (c *APIController) GetAllCreatedTopics() {
	author := c.Input().Get("id")
	tab := c.Input().Get("tab")
	limitStr := c.Input().Get("limit")
	pageStr := c.Input().Get("page")
	var (
		limit, offset int
		err           error
	)
	if len(limitStr) != 0 {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			panic(err)
		}
	} else {
		limit = 10
	}
	if len(pageStr) != 0 {
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			panic(err)
		}
		offset = page*limit - limit
	}

	c.Data["json"] = object.GetAllCreatedTopics(author, tab, limit, offset)
	c.ServeJSON()
}

func (c *APIController) GetCreatedTopicsNum() {
	memberId := c.Input().Get("id")

	c.Data["json"] = object.GetCreatedTopicsNum(memberId)
	c.ServeJSON()
}

func (c *APIController) GetTopicsByNode() {
	nodeId := c.Input().Get("node-id")
	limitStr := c.Input().Get("limit")
	pageStr := c.Input().Get("page")
	defaultLimit := object.DefaultPageNum

	var limit, offset int
	if len(limitStr) != 0 {
		limit = util.ParseInt(limitStr)
	} else {
		limit = defaultLimit
	}
	if len(pageStr) != 0 {
		page := util.ParseInt(pageStr)
		offset = page*limit - limit
	}

	c.Data["json"] = object.GetTopicsWithNode(nodeId, limit, offset)
	c.ServeJSON()
}

//together with node
func (c *APIController) AddTopicHitCount() {
	topicIdStr := c.Input().Get("id")

	var resp Response
	topicId := util.ParseInt(topicIdStr)
	res := object.AddTopicHitCount(topicId)
	topicInfo := object.GetTopic(topicId)
	hitRecord := object.BrowseRecord{
		MemberId:    c.GetSessionUser(),
		RecordType:  1,
		ObjectId:    topicInfo.NodeId,
		CreatedTime: util.GetCurrentTime(),
		Expired:     false,
	}
	object.AddBrowseRecordNum(&hitRecord)
	if res {
		resp = Response{Status: "ok", Msg: "success"}
	} else {
		resp = Response{Status: "fail", Msg: "add topic hit count failed"}
	}

	c.Data["json"] = resp
	c.ServeJSON()
}

func (c *APIController) GetTopicsByTab() {
	tabId := c.Input().Get("tab-id")
	limitStr := c.Input().Get("limit")
	pageStr := c.Input().Get("page")
	defaultLimit := object.DefaultHomePageNum

	var limit, offset int
	if len(limitStr) != 0 {
		limit = util.ParseInt(limitStr)
	} else {
		limit = defaultLimit
	}
	if len(pageStr) != 0 {
		page := util.ParseInt(pageStr)
		offset = page*limit - limit
	}

	c.Data["json"] = object.GetTopicsWithTab(tabId, limit, offset)
	c.ServeJSON()
}

func (c *APIController) AddTopicBrowseCount() {
	topicId := c.Input().Get("id")

	var resp Response
	hitRecord := object.BrowseRecord{
		MemberId:    c.GetSessionUser(),
		RecordType:  2,
		ObjectId:    topicId,
		CreatedTime: util.GetCurrentTime(),
		Expired:     false,
	}
	res := object.AddBrowseRecordNum(&hitRecord)
	if res {
		resp = Response{Status: "ok", Msg: "success"}
	} else {
		resp = Response{Status: "fail", Msg: "add node hit count failed"}
	}

	c.Data["json"] = resp
	c.ServeJSON()
}

func (c *APIController) GetHotTopic() {
	limitStr := c.Input().Get("limit")
	defaultLimit := object.HotTopicNum

	var limit int
	if len(limitStr) != 0 {
		limit = util.ParseInt(limitStr)
	} else {
		limit = defaultLimit
	}

	var resp Response
	res := object.GetHotTopic(limit)
	resp = Response{Status: "ok", Msg: "success", Data: res}

	c.Data["json"] = resp
	c.ServeJSON()
}

func (c *APIController) UpdateTopicNode() {
	if c.RequireLogin() {
		return
	}

	var resp Response
	memberId := c.GetSessionUser()
	var form updateTopicNode
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &form)
	if err != nil {
		panic(err)
	}
	id, nodeName, nodeId := form.Id, form.NodeName, form.NodeId

	originalNode := object.GetTopicNodeId(id)
	if !object.CheckModIdentity(memberId) && !object.CheckNodeModerator(memberId, originalNode) && object.GetTopicAuthor(id) != memberId {
		resp = Response{Status: "fail", Msg: "Unauthorized."}
		c.Data["json"] = resp
		c.ServeJSON()
		return
	}

	topic := object.Topic{
		//Id:       id,
		NodeId:   nodeId,
		NodeName: nodeName,
	}
	res := object.UpdateTopicWithLimitCols(id, &topic)

	resp = Response{Status: "ok", Msg: "success", Data: res}
	c.Data["json"] = resp
	c.ServeJSON()
}

func (c *APIController) EditContent() {
	if c.RequireLogin() {
		return
	}

	editType := c.Input().Get("editType")
	var resp Response
	memberId := c.GetSessionUser()
	if editType == "topic" {
		var form editTopic
		err := json.Unmarshal(c.Ctx.Input.RequestBody, &form)
		if err != nil {
			panic(err)
		}
		id, title, content, nodeId, editorType := form.Id, form.Title, form.Content, form.NodeId, form.EditorType
		if !object.CheckModIdentity(memberId) && !object.CheckNodeModerator(memberId, nodeId) && object.GetTopicAuthor(id) != memberId {
			resp = Response{Status: "fail", Msg: "Unauthorized."}
			c.Data["json"] = resp
			c.ServeJSON()
			return
		}

		topic := object.Topic{
			Id:         id,
			Title:      title,
			Content:    content,
			EditorType: editorType,
		}
		res := object.UpdateTopicWithLimitCols(id, &topic)

		resp = Response{Status: "ok", Msg: "success", Data: res}
	} else {
		var form editReply
		err := json.Unmarshal(c.Ctx.Input.RequestBody, &form)
		if err != nil {
			panic(err)
		}
		id, content, editorType := form.Id, form.Content, form.EditorType
		if !object.CheckModIdentity(memberId) && object.GetReplyAuthor(id) != memberId {
			resp = Response{Status: "fail", Msg: "Unauthorized."}
			c.Data["json"] = resp
			c.ServeJSON()
			return
		}

		reply := object.Reply{
			Id:         id,
			Content:    content,
			EditorType: editorType,
		}
		res := object.UpdateReplyWithLimitCols(id, &reply)

		resp = Response{Status: "ok", Msg: "success", Data: res}
	}

	c.Data["json"] = resp
	c.ServeJSON()
}

// TopTopic tops topic according to the topType in the url.
func (c *APIController) TopTopic() {
	if c.RequireLogin() {
		return
	}

	idStr := c.Input().Get("id")
	memberId := c.GetSessionUser()

	id := util.ParseInt(idStr)
	var resp Response
	var res bool

	nodeId := object.GetTopicNodeId(id)
	if object.CheckModIdentity(memberId) || object.CheckNodeModerator(memberId, nodeId) {
		//timeStr := c.Input().Get("time")
		//time := util.ParseInt(timeStr)
		//date := util.GetTimeMinute(time)
		//res = object.ChangeTopicTopExpiredTime(id, date)
		topType := c.Input().Get("topType")
		date := util.GetTimeYear(100)
		res = object.ChangeTopicTopExpiredTime(id, date, topType)
	} else if object.GetTopicAuthor(id) == memberId {
		balance := object.GetMemberBalance(memberId)
		if balance < object.TopTopicCost {
			resp = Response{Status: "fail", Msg: "You don't have enough balance."}
			c.Data["json"] = resp
			c.ServeJSON()
			return
		}
		object.TopTopicConsumption(memberId, id)
		date := util.GetTimeMinute(object.DefaultTopTopicTime)
		res = object.ChangeTopicTopExpiredTime(id, date, "node")
	} else {
		resp = Response{Status: "fail", Msg: "Unauthorized."}
		c.Data["json"] = resp
		c.ServeJSON()
		return
	}

	resp = Response{Status: "ok", Msg: "success", Data: res}
	c.Data["json"] = resp
	c.ServeJSON()
}

// CancelTopTopic cancels top topic according to the topType in the url.
func (c *APIController) CancelTopTopic() {
	if c.RequireLogin() {
		return
	}

	idStr := c.Input().Get("id")
	memberId := c.GetSessionUser()

	id := util.ParseInt(idStr)
	var resp Response
	var res bool

	nodeId := object.GetTopicNodeId(id)
	if object.CheckModIdentity(memberId) || object.CheckNodeModerator(memberId, nodeId) {
		topType := c.Input().Get("topType")
		res = object.ChangeTopicTopExpiredTime(id, "", topType)
	} else {
		resp = Response{Status: "fail", Msg: "Unauthorized."}
		c.Data["json"] = resp
		c.ServeJSON()
		return
	}

	resp = Response{Status: "ok", Msg: "success", Data: res}
	c.Data["json"] = resp
	c.ServeJSON()
}
