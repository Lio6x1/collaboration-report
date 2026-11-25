export function importMarkdown(text:string,options?:{newId?:()=>string;resources?:Map<string,string>;workspaceKind?:string}):{body:any;losses:string[];externalImages:Array<{url:string;alt:string}>};
