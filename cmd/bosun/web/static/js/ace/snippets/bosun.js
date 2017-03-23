ace.define("ace/snippets/bosun",["require","exports","module"], function(require, exports, module) {

exports.snippets = [
    /* Sections */
    {
        name: "alert definition",
        tabTrigger: "alertdef",
        content: 
"alert ${1:alertname} {\n\
    warn = ${2:numberSetExpression}\n\
}\n"
    },
    {
        name: "template def",
        tabTrigger: "templatedef",
        content: 
"template ${1:templatename} {\n\
    subject =\n\
}\n"
    },

    /* Reduction Funcs */
    {
        name: "avg reduction function",
        tabTrigger: "avg",
        content: "avg(${1:seriesSet})"
    },

    /* Elastic Funcs */
    {
        name: "escount elastic function",
        tabTrigger: "escount",
        content: "escount(${1:indexer}, \"${2:keysCSV}\", ${3:filter}, \"${4:bucketDuration}\", \"${5:startDuration}\", \"${6:endDuration}\")"
    }
]

exports.scope = "bosun";

});
